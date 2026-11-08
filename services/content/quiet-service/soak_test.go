package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http/httptest"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
)

type soakClient struct {
	conn          *websocket.Conn
	id, roomID    string
	mu            sync.Mutex
	latest        event
	frames        int
	serverErrors  []string
	readErrors    []string
	expectedClose atomic.Bool
	closeOnce     sync.Once
	done          chan struct{}
}

type soakLatency struct {
	mu        sync.Mutex
	probeID   string
	sent      int
	pending   map[float64]time.Time
	latencies []float64
}

func (m *soakLatency) observe(e event, received time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, player := range e.Players {
		if player.ID != m.probeID {
			continue
		}
		if sent, ok := m.pending[player.Yaw]; ok {
			m.latencies = append(m.latencies, float64(received.Sub(sent).Microseconds())/1000)
			delete(m.pending, player.Yaw)
		}
	}
}

func connectSoakClient(t *testing.T, b *browser, observe func(event, time.Time)) *soakClient {
	t.Helper()
	conn := b.dial(t)
	welcome := readUntil(t, conn, func(e event) bool { return e.Type == "welcome" })
	client := &soakClient{conn: conn, id: welcome.ID, roomID: welcome.RoomID, latest: welcome, done: make(chan struct{})}
	go func() {
		defer close(client.done)
		for {
			_, raw, err := conn.Read(context.Background())
			if err != nil {
				if !client.expectedClose.Load() {
					client.mu.Lock()
					client.readErrors = append(client.readErrors, err.Error())
					client.mu.Unlock()
				}
				return
			}
			var message event
			if err := json.Unmarshal(raw, &message); err != nil {
				client.mu.Lock()
				client.readErrors = append(client.readErrors, "invalid JSON state")
				client.mu.Unlock()
				continue
			}
			client.mu.Lock()
			client.frames++
			if message.Type == "state" {
				client.latest = message
			} else if message.Type == "error" {
				client.serverErrors = append(client.serverErrors, message.Message)
			}
			client.mu.Unlock()
			if observe != nil && message.Type == "state" {
				observe(message, time.Now())
			}
		}
	}()
	return client
}

func (c *soakClient) close() {
	c.closeOnce.Do(func() {
		c.expectedClose.Store(true)
		_ = c.conn.CloseNow()
		<-c.done
	})
}

func (c *soakClient) snapshot() event {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.latest
}

func (c *soakClient) self(t *testing.T) player {
	t.Helper()
	for _, player := range c.snapshot().Players {
		if player.ID == c.id {
			return player
		}
	}
	t.Fatal("own player missing from WebSocket state")
	return player{}
}

func (c *soakClient) write(value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	return c.conn.Write(ctx, websocket.MessageText, raw)
}

func soakWait(t *testing.T, label string, check func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !check() {
		if time.Now().After(deadline) {
			t.Fatal("timed out: " + label)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func soakOccupancy(r *room) (int, int, string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	seats := 0
	for _, id := range r.seats {
		if id != "" {
			seats++
		}
	}
	return len(r.players), seats, r.seats[0]
}

func percentile(values []float64, proportion float64) float64 {
	if len(values) == 0 {
		return 0
	}
	return values[max(0, int(math.Ceil(float64(len(values))*proportion))-1)]
}

func soakFinalPositionsMatch(snapshot event, expected []player) bool {
	if len(snapshot.Players) != len(expected) {
		return false
	}
	for _, final := range expected {
		found := false
		for _, actual := range snapshot.Players {
			if actual.ID == final.ID && final.ID != "" && math.Abs(actual.X-final.X) < 1e-9 && math.Abs(actual.Z-final.Z) < 1e-9 && math.Abs(actual.Yaw-final.Yaw) < 1e-9 {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func TestSoakFinalPositionsMatch(t *testing.T) {
	expected := []player{{ID: "one", X: 2, Z: -3, Yaw: .7}, {ID: "two", X: -1, Z: 4, Yaw: 1.2}}
	if !soakFinalPositionsMatch(event{Players: []player{expected[1], expected[0]}}, expected) {
		t.Fatal("snapshot order must not affect convergence")
	}
	if soakFinalPositionsMatch(event{Players: expected[:1]}, expected) {
		t.Fatal("a missing player must fail convergence")
	}
	stale := append([]player(nil), expected...)
	stale[1].Yaw -= .001
	if soakFinalPositionsMatch(event{Players: stale}, expected) {
		t.Fatal("an older intermediate pose must fail convergence")
	}
	stale[1] = expected[1]
	stale[1].X += .001
	if soakFinalPositionsMatch(event{Players: stale}, expected) {
		t.Fatal("matching yaw alone is insufficient")
	}
}

// TestLocalTwentyClientSoak uses the real app HTTP/WS handlers on a private loopback listener.
// It does not use the preview server, persistence services, browsers or 3D renderers.
func TestLocalTwentyClientSoak(t *testing.T) {
	if os.Getenv("QUIET_TEST_SOAK") != "1" {
		t.Skip("opt-in 90s same-host WebSocket soak; set QUIET_TEST_SOAK=1")
	}
	started := time.Now()
	stage, reportWritten := "setup", false
	metrics := &soakLatency{pending: map[float64]time.Time{}}
	defer func() {
		if !t.Failed() || reportWritten {
			return
		}
		metrics.mu.Lock()
		failure := map[string]any{"passed": false, "failureStage": stage, "durationSeconds": time.Since(started).Seconds(), "probeSent": metrics.sent, "probeObserved": len(metrics.latencies), "probeMissing": len(metrics.pending), "scope": "isolated same-host memory hub; see raw log for failed assertion"}
		metrics.mu.Unlock()
		raw, _ := json.MarshalIndent(failure, "", "  ")
		if path := os.Getenv("QUIET_SOAK_REPORT"); path != "" {
			_ = os.WriteFile(path, append(raw, '\n'), 0600)
		}
		t.Logf("SOAK_FAILURE_REPORT %s", raw)
	}()
	store := memoryStorage()
	service, err := newApp(store, "isolated-soak-test")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(service.handler())
	var all []*soakClient
	t.Cleanup(func() {
		for _, client := range all {
			client.close()
		}
		service.close()
		srv.Close()
		store.close()
	})
	stage = "twenty_members"
	browsers, clients := make([]*browser, capacity), make([]*soakClient, capacity)
	for i := range clients {
		browsers[i] = newBrowser(t, srv.URL, false)
		var observe func(event, time.Time)
		if i == capacity-1 {
			observe = metrics.observe
		}
		clients[i] = connectSoakClient(t, browsers[i], observe)
		all = append(all, clients[i])
		if clients[i].roomID != clients[0].roomID {
			t.Fatal("20 guests did not join the same isolated room")
		}
	}
	service.mu.Lock()
	activeRoom := service.rooms[0]
	service.mu.Unlock()
	waitForTwenty := func() {
		soakWait(t, "20 members in authoritative room and all client snapshots", func() bool {
			count, _, _ := soakOccupancy(activeRoom)
			if count != capacity {
				return false
			}
			for _, client := range clients {
				if len(client.snapshot().Players) != capacity {
					return false
				}
			}
			return true
		})
	}
	waitForTwenty()
	t.Log("20 independent guest-cookie clients joined one isolated real HTTP/WebSocket room")

	positions := []player{clients[0].self(t), clients[1].self(t)}
	stage = "seat_race_and_reconnect"
	for complete := false; !complete; {
		time.Sleep(100 * time.Millisecond)
		complete = true
		for index := range positions {
			p := &positions[index]
			dx, dz := -2-p.X, -2-p.Z
			distance := math.Hypot(dx, dz)
			if distance < .001 {
				continue
			}
			complete = false
			step := math.Min(.28, distance)
			p.X, p.Z, p.Yaw = p.X+dx/distance*step, p.Z+dz/distance*step, math.Atan2(dx, dz)
			if err := clients[index].write(map[string]any{"type": "move", "x": p.X, "z": p.Z, "yaw": p.Yaw}); err != nil {
				t.Fatal(err)
			}
		}
	}
	soakWait(t, "both contenders arrived via validated movement", func() bool {
		return math.Hypot(clients[0].self(t).X+2, clients[0].self(t).Z+2) < .01 && math.Hypot(clients[1].self(t).X+2, clients[1].self(t).Z+2) < .01
	})
	var race sync.WaitGroup
	barrier, writes := make(chan struct{}), make(chan error, 2)
	for i := 0; i < 2; i++ {
		race.Add(1)
		go func(client *soakClient) {
			defer race.Done()
			<-barrier
			writes <- client.write(map[string]any{"type": "sit", "seat": 0})
		}(clients[i])
	}
	close(barrier)
	race.Wait()
	for i := 0; i < 2; i++ {
		if err := <-writes; err != nil {
			t.Fatal(err)
		}
	}
	soakWait(t, "one authoritative seat winner", func() bool { _, seats, owner := soakOccupancy(activeRoom); return seats == 1 && owner != "" })
	_, _, owner := soakOccupancy(activeRoom)
	winner := 0
	if owner == clients[1].id {
		winner = 1
	} else if owner != clients[0].id {
		t.Fatal("seat granted to a non-contender")
	}
	loser := 1 - winner
	soakWait(t, "loser received seat-conflict frame", func() bool {
		clients[loser].mu.Lock()
		defer clients[loser].mu.Unlock()
		return len(clients[loser].serverErrors) == 1 && strings.Contains(clients[loser].serverErrors[0], "已有人")
	})
	oldID := clients[winner].id
	clients[winner].close()
	soakWait(t, "disconnect released membership and occupied seat", func() bool { count, seats, _ := soakOccupancy(activeRoom); return count == 19 && seats == 0 })
	if err := clients[loser].write(map[string]any{"type": "sit", "seat": 0}); err != nil {
		t.Fatal(err)
	}
	soakWait(t, "previous loser acquired the released seat", func() bool { _, _, owner := soakOccupancy(activeRoom); return owner == clients[loser].id })
	if err := clients[loser].write(map[string]any{"type": "stand"}); err != nil {
		t.Fatal(err)
	}
	soakWait(t, "stand released seat", func() bool { _, seats, _ := soakOccupancy(activeRoom); return seats == 0 })
	clients[winner] = connectSoakClient(t, browsers[winner], nil)
	all = append(all, clients[winner])
	if clients[winner].id == oldID || clients[winner].roomID != clients[loser].roomID || clients[winner].self(t).Seat != nil {
		t.Fatal("reconnection reused old identity/seat or joined wrong room")
	}
	waitForTwenty()
	t.Log("seat race, transport loss, seat release/reclaim and fresh-ID reconnect verified through actual state")

	metrics.mu.Lock()
	metrics.probeID = clients[0].id
	metrics.mu.Unlock()
	runStart := time.Now()
	stage = "steady_load"
	finalPositions := make([]player, capacity)
	ctx, cancel := context.WithCancel(context.Background())
	var workers sync.WaitGroup
	var commandCount atomic.Int64
	writeErrors := make(chan error, capacity)
	for index, client := range clients {
		base := client.self(t)
		workers.Add(1)
		go func(index int, client *soakClient, base player) {
			defer workers.Done()
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			sequence := 0
			phase := float64(index) * .31
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if ctx.Err() != nil {
						return
					}
					sequence++
					elapsed := time.Since(runStart).Seconds()
					x, z, yaw := base.X+.6*(math.Sin(elapsed*.7+phase)-math.Sin(phase)), base.Z+.6*(math.Cos(elapsed*.7+phase)-math.Cos(phase)), float64(sequence)*.001
					if index == 0 {
						metrics.mu.Lock()
						metrics.sent++
						metrics.pending[yaw] = time.Now()
						metrics.mu.Unlock()
					}
					if err := client.write(map[string]any{"type": "move", "x": x, "z": z, "yaw": yaw}); err != nil {
						writeErrors <- err
						return
					}
					commandCount.Add(1)
					finalPositions[index] = player{ID: client.id, X: x, Z: z, Yaw: yaw}
				}
			}
		}(index, client, base)
	}
	for segment := 1; segment <= 6; segment++ {
		time.Sleep(15 * time.Second)
		count, seats, _ := soakOccupancy(activeRoom)
		if count != capacity || seats != 0 {
			cancel()
			workers.Wait()
			t.Fatalf("soak membership changed: members=%d seats=%d", count, seats)
		}
		t.Logf("steady segment %d/6: elapsed=%.1fs members=20 commands=%d", segment, time.Since(runStart).Seconds(), commandCount.Load())
	}
	cancel()
	workers.Wait()
	steadySeconds := time.Since(runStart).Seconds()
	close(writeErrors)
	writeErrorCount := 0
	for err := range writeErrors {
		writeErrorCount++
		t.Error("soak write failed", err)
	}
	stage = "final_positions_converge"
	convergenceStart := time.Now()
	// Intermediate states may be coalesced; every reader must receive each player's final accepted pose.
	soakWait(t, "all 20 clients see all 20 final positions", func() bool {
		for _, client := range clients {
			if !soakFinalPositionsMatch(client.snapshot(), finalPositions) {
				return false
			}
		}
		return true
	})
	convergenceSeconds := time.Since(convergenceStart).Seconds()
	waitForTwenty()

	stage = "post_soak_reconnect_and_overflow"
	oldID = clients[7].id
	clients[7].close()
	soakWait(t, "post-soak disconnect removed member", func() bool { count, _, _ := soakOccupancy(activeRoom); return count == 19 })
	clients[7] = connectSoakClient(t, browsers[7], nil)
	all = append(all, clients[7])
	if clients[7].id == oldID {
		t.Fatal("post-soak reconnect retained old connection identity")
	}
	waitForTwenty()
	overflow := connectSoakClient(t, newBrowser(t, srv.URL, false), nil)
	all = append(all, overflow)
	if overflow.roomID == clients[0].roomID || len(overflow.snapshot().Players) != 1 {
		t.Fatal("21st client was not isolated in another room")
	}
	overflow.close()
	stage = "cleanup"
	for _, client := range clients {
		client.close()
	}
	soakWait(t, "all isolated rooms have zero members and zero occupied seats", func() bool {
		service.mu.Lock()
		defer service.mu.Unlock()
		for _, r := range service.rooms {
			count, seats, _ := soakOccupancy(r)
			if count != 0 || seats != 0 {
				return false
			}
		}
		return true
	})
	service.snapshotMu.Lock()
	for _, snapshot := range service.snapshots {
		if snapshot.Count != 0 {
			t.Errorf("published cleanup count was not zero: %d", snapshot.Count)
		}
	}
	service.snapshotMu.Unlock()
	frames, serverErrors, readErrors := 0, 0, 0
	for _, client := range all {
		client.mu.Lock()
		frames += client.frames
		serverErrors += len(client.serverErrors)
		readErrors += len(client.readErrors)
		client.mu.Unlock()
	}
	if serverErrors != 1 || readErrors != 0 {
		t.Errorf("unexpected errors: server=%d (one expected seat conflict), reads=%d", serverErrors, readErrors)
	}
	if !t.Failed() {
		stage = "complete"
	}
	metrics.mu.Lock()
	sort.Float64s(metrics.latencies)
	report := map[string]any{
		"scope":     "same-host real TCP HTTP/WebSocket clients, isolated httptest newApp memory hub; no preview-server, browser/GPU, WSS, MySQL or Redis capacity claim",
		"startedAt": started.UTC().Format(time.RFC3339Nano), "durationSeconds": time.Since(started).Seconds(), "steadySeconds": steadySeconds,
		"simultaneousClients": capacity, "targetMoveHzPerClient": 10, "commandsSent": commandCount.Load(), "clientFramesRead": frames,
		"probeMetric": "send to first matching state seen by another WebSocket client; observed samples only, intermediate states may be coalesced; not per-message delivery, frame rendering or internet latency",
		"probeSent":   metrics.sent, "probeObserved": len(metrics.latencies), "probeMissing": len(metrics.pending),
		"stateLatencyMs":        map[string]float64{"p50": percentile(metrics.latencies, .5), "p95": percentile(metrics.latencies, .95), "p99": percentile(metrics.latencies, .99), "max": percentile(metrics.latencies, 1)},
		"expectedSeatConflicts": serverErrors, "unexpectedReadErrors": readErrors, "unexpectedWriteErrors": writeErrorCount,
		"seatRace":           "one winner; loser rejected; disconnected owner released seat; loser then acquired and stood",
		"reconnectsVerified": 2, "twentyFirstClient": "new room with one member", "remainingMembers": 0, "remainingOccupiedSeats": 0,
		"connectionsCreated": len(all), "goVersion": runtime.Version(), "gomaxprocs": runtime.GOMAXPROCS(0), "passed": !t.Failed(),
		"instrumentation": "normal go test build, not a browser or -race benchmark", "cleanupBasis": "authoritative in-memory rooms and app-published snapshots; no shared persistence was touched",
		"finalPositionsConverged": true, "finalPositionsPerClient": capacity, "convergenceSeconds": convergenceSeconds,
		"failureStage": stage,
	}
	metrics.mu.Unlock()
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if path := os.Getenv("QUIET_SOAK_REPORT"); path != "" {
		if err := os.WriteFile(path, append(raw, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	reportWritten = true
	t.Logf("SOAK_REPORT %s", raw)
	t.Log(fmt.Sprintf("cleanup verified: 0 members, 0 occupied seats; %d connection lifetimes", len(all)))
}

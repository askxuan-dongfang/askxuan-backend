package main

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func testPeer(t *testing.T, r *room, now time.Time) *peer {
	t.Helper()
	p, _, err := r.join(nil, func() {}, now)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestSeatOwnershipAndLeave(t *testing.T) {
	r := newRoom()
	now := time.Now()
	a, b := testPeer(t, r, now), testPeer(t, r, now)
	a.X, a.Z, b.X, b.Z = -2, -2, -2, -2
	if err := r.command(a.ID, []byte(`{"type":"sit","seat":0}`), now); err != nil {
		t.Fatal(err)
	}
	if a.Yaw != math.Pi {
		t.Fatal("seated visitors must face the temple along negative z")
	}
	if err := r.command(b.ID, []byte(`{"type":"sit","seat":0}`), now); err == nil {
		t.Fatal("a second visitor occupied the same seat")
	}
	if err := r.command(a.ID, []byte(`{"type":"action","action":"greet"}`), now); err != nil {
		t.Fatal(err)
	}
	if err := r.command(a.ID, []byte(`{"type":"action","action":"idle"}`), now); err != nil || a.Action != "sit" {
		t.Fatal("finishing a seated gesture must return to sitting", err)
	}
	r.leave(a.ID)
	if err := r.command(b.ID, []byte(`{"type":"sit","seat":0}`), now); err != nil {
		t.Fatal("disconnection did not release seat", err)
	}
	if len(r.players) != 1 || r.seats[0] != b.ID {
		t.Fatal("room membership or seat owner is incorrect")
	}
	if err := r.command(b.ID, []byte(`{"type":"stand"}`), now); err != nil || b.Seat != nil || r.seats[0] != "" {
		t.Fatal("standing did not release seat", err)
	}
}

func TestRejectInvalidCommandsAndTeleport(t *testing.T) {
	for _, data := range []string{
		`{`, `{"type":"stand"}{"type":"stand"}`, `{"type":"stand","id":"other"}`,
		`{"type":"move","x":0}`, `{"type":"move","x":100,"z":5,"yaw":0}`,
		`{"type":"move","x":0,"z":5,"yaw":1e309}`, `{"type":"move","x":0,"z":5,"yaw":7}`,
		`{"type":"move","x":0,"z":-2,"yaw":0}`, `{"type":"action","action":"sit"}`,
		`{"type":"action","action":"admin"}`, `{"type":"sit","seat":0}`, `{"type":"sit","seat":6}`,
		strings.Repeat(" ", 1025),
	} {
		t.Run(data[:min(len(data), 60)], func(t *testing.T) {
			r := newRoom()
			now := time.Now()
			p := testPeer(t, r, now)
			if err := r.command(p.ID, []byte(data), now); err == nil {
				t.Fatalf("accepted invalid input: %s", data)
			}
			if p.X != 0 || p.Z != 5 || p.Seat != nil || p.Action != "idle" {
				t.Fatal("invalid command mutated player")
			}
		})
	}
}

func TestMovementCreditAndRateLimit(t *testing.T) {
	r := newRoom()
	now := time.Now()
	p := testPeer(t, r, now)
	if err := r.command(p.ID, []byte(`{"type":"move","x":1,"z":5,"yaw":1}`), now); err != nil {
		t.Fatal(err)
	}
	if err := r.command(p.ID, []byte(`{"type":"move","x":2,"z":5,"yaw":1}`), now); err == nil {
		t.Fatal("back-to-back moves bypassed speed limit")
	}
	if err := r.command(p.ID, []byte(`{"type":"move","x":2,"z":5,"yaw":1}`), now.Add(time.Second)); err != nil {
		t.Fatal("movement credit did not refill", err)
	}
	for i := 0; i < 40; i++ {
		_ = r.command(p.ID, []byte(`{"type":"action","action":"greet"}`), now.Add(time.Second))
	}
	if err := r.command(p.ID, []byte(`{"type":"stand"}`), now.Add(time.Second)); err == nil {
		t.Fatal("unbounded message burst was accepted")
	}
	if err := r.command(p.ID, []byte(`{"type":"stand"}`), now.Add(2*time.Second)); err != nil {
		t.Fatal("rate limit did not refill", err)
	}
}

func TestCapacityAndBoundedStateQueue(t *testing.T) {
	r := newRoom()
	now := time.Now()
	for i := 0; i < capacity; i++ {
		testPeer(t, r, now)
	}
	if _, _, err := r.join(nil, func() {}, now); err == nil {
		t.Fatal("room exceeded capacity")
	}
	for _, p := range r.players {
		if len(p.states) != 1 {
			t.Fatal("slow clients should retain only the latest state")
		}
	}
}

type event struct {
	Type     string   `json:"type"`
	ID       string   `json:"id"`
	Players  []player `json:"players"`
	Capacity int      `json:"capacity"`
	Message  string   `json:"message"`
	RoomID   string   `json:"roomId"`
}

func TestHallOfferAndInteriorSeat(t *testing.T) {
	r := newRoom()
	now := time.Now()
	a, b := testPeer(t, r, now), testPeer(t, r, now)
	for _, data := range []string{`{"type":"offer","hall":"unknown"}`, `{"type":"offer","hall":"han-main"}`, `{"type":"sit","seat":30}`} {
		if err := r.command(a.ID, []byte(data), now); err == nil {
			t.Fatal("invalid hall operation accepted", data)
		}
	}
	a.X, a.Z = hallApproaches["han-main"][0], hallApproaches["han-main"][1]
	if err := r.command(a.ID, []byte(`{"type":"offer","hall":"han-main"}`), now); err != nil || a.OfferingHall == nil || *a.OfferingHall != "han-main" || a.Action != "greet" || a.Yaw != math.Pi {
		t.Fatal("hall offer missing authoritative state", err)
	}
	if err := r.command(a.ID, []byte(`{"type":"action","action":"idle"}`), now); err != nil || a.OfferingHall != nil {
		t.Fatal("idle did not clear offering", err)
	}
	a.X, a.Z, b.X, b.Z = seatPositions[6][0], seatPositions[6][1], seatPositions[6][0], seatPositions[6][1]
	if err := r.command(a.ID, []byte(`{"type":"sit","seat":6}`), now); err != nil {
		t.Fatal(err)
	}
	if err := r.command(b.ID, []byte(`{"type":"sit","seat":6}`), now); err == nil {
		t.Fatal("hall seat occupied twice")
	}
	if err := r.command(a.ID, []byte(`{"type":"offer","hall":"han-main"}`), now); err == nil {
		t.Fatal("seated visitor could offer")
	}
	if err := r.command(a.ID, []byte(`{"type":"stand"}`), now); err != nil || a.OfferingHall != nil {
		t.Fatal("stand did not clear offering", err)
	}
}

func readUntil(t *testing.T, c *websocket.Conn, match func(event) bool) event {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	for {
		_, data, err := c.Read(ctx)
		if err != nil {
			t.Fatal(err)
		}
		var e event
		if err := json.Unmarshal(data, &e); err != nil {
			t.Fatal(err)
		}
		if match(e) {
			return e
		}
	}
}

func TestWebSocketTwoClientsAndOrigin(t *testing.T) {
	r := newRoom()
	srv := httptest.NewServer(r)
	defer srv.Close()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/quiet-ws"
	for _, origin := range []string{"", "https://evil.example", "http://localhost:5186.evil.example", "http://localhost:9999"} {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		c, response, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{origin}}})
		cancel()
		if c != nil {
			c.CloseNow()
		}
		if err == nil || response == nil || response.StatusCode != http.StatusForbidden {
			t.Fatalf("untrusted origin accepted: %q", origin)
		}
	}
	dial := func() *websocket.Conn {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		c, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: http.Header{"Origin": []string{"http://localhost:5186"}}})
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	a := dial()
	defer a.CloseNow()
	first := readUntil(t, a, func(e event) bool { return e.Type == "welcome" })
	if first.ID == "" || first.Capacity != capacity || len(first.Players) != 1 {
		t.Fatal("invalid welcome", first)
	}
	b := dial()
	defer b.CloseNow()
	second := readUntil(t, b, func(e event) bool { return e.Type == "welcome" })
	if second.ID == first.ID || len(second.Players) != 2 {
		t.Fatal("visitors did not receive independent identities and shared state")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := a.Write(ctx, websocket.MessageText, []byte(`{"type":"action","action":"greet"}`)); err != nil {
		t.Fatal(err)
	}
	readUntil(t, b, func(e event) bool {
		for _, p := range e.Players {
			if p.ID == first.ID && p.Action == "greet" {
				return true
			}
		}
		return false
	})
	if err := a.Write(ctx, websocket.MessageText, []byte(`{"type":"move","x":99,"z":5,"yaw":0}`)); err != nil {
		t.Fatal(err)
	}
	readUntil(t, a, func(e event) bool { return e.Type == "error" && e.Message != "" })
	_ = a.CloseNow()
	readUntil(t, b, func(e event) bool { return e.Type == "state" && len(e.Players) == 1 && e.Players[0].ID == second.ID })
}

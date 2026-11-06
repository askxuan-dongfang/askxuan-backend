package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
)

const capacity = 20

type player struct {
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	X            float64 `json:"x"`
	Z            float64 `json:"z"`
	Yaw          float64 `json:"yaw"`
	Action       string  `json:"action"`
	Seat         *int    `json:"seat"`
	OfferingHall *string `json:"offeringHall"`
}

type peer struct {
	player
	conn       *websocket.Conn
	cancel     context.CancelFunc
	states     chan []byte
	errors     chan []byte
	moveAt     time.Time
	moveCredit float64
	rateAt     time.Time
	rateCredit float64
}

type room struct {
	origins []string
	mu      sync.Mutex
	players map[string]*peer
	seats   [30]string
	serial  uint64
	id      string
	publish func(string, int, []byte)
}

func newRoom() *room { return &room{players: make(map[string]*peer)} }

func parseOrigins(raw string) ([]string, error) {
	if raw == "" {
		return nil, nil
	}
	origins := strings.Split(raw, ",")
	for i, origin := range origins {
		origin = strings.TrimSpace(origin)
		u, err := url.Parse(origin)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.String() != origin || strings.ContainsAny(origin, "*\\") {
			return nil, errors.New("QUIET_PUBLIC_ORIGINS 只允许精确HTTPS origin，以逗号分隔")
		}
		origins[i] = origin
	}
	return origins, nil
}

func allowedOrigin(origin string, configured []string) bool {
	switch origin {
	case "http://localhost:5186", "http://127.0.0.1:5186", "http://localhost:5173", "http://127.0.0.1:5173", "https://localhost:8446", "https://127.0.0.1:8446":
		return true
	}
	for _, permitted := range configured {
		if origin == permitted {
			return true
		}
	}
	return false
}

func (r *room) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.serve(w, req, nil)
}

func (r *room) serve(w http.ResponseWriter, req *http.Request, joined func()) {
	if req.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	// The app requires a server-issued guest session before reserving a room.
	if !allowedOrigin(req.Header.Get("Origin"), r.origins) {
		http.Error(w, "只允许配置的同源入口", http.StatusForbidden)
		return
	}
	c, err := websocket.Accept(w, req, &websocket.AcceptOptions{OriginPatterns: []string{req.Header.Get("Origin")}})
	if err != nil {
		return
	}
	defer c.CloseNow()
	c.SetReadLimit(1024)
	ctx, cancel := context.WithCancel(req.Context())
	defer cancel()
	p, welcome, err := r.join(c, cancel, time.Now())
	if err != nil {
		_ = c.Close(websocket.StatusTryAgainLater, "本地体验房已满，请稍后重试")
		return
	}
	defer r.leave(p.ID)
	if joined != nil {
		joined()
	}
	writeCtx, done := context.WithTimeout(ctx, 3*time.Second)
	err = c.Write(writeCtx, websocket.MessageText, welcome)
	done()
	if err != nil {
		return
	}
	go p.write(ctx)
	for {
		kind, data, err := c.Read(ctx)
		if err != nil {
			return
		}
		if kind != websocket.MessageText {
			_ = c.Close(websocket.StatusUnsupportedData, "只接受JSON文本消息")
			return
		}
		if err := r.command(p.ID, data, time.Now()); err != nil {
			payload, _ := json.Marshal(struct {
				Type    string `json:"type"`
				Message string `json:"message"`
			}{"error", err.Error()})
			select {
			case p.errors <- payload:
			default:
				return
			}
		}
	}
}

func (p *peer) write(ctx context.Context) {
	defer p.cancel()
	defer p.conn.CloseNow()
	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()
	for {
		var payload []byte
		select {
		case <-ctx.Done():
			return
		case payload = <-p.states:
		case payload = <-p.errors:
		case <-ping.C:
			pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := p.conn.Ping(pingCtx)
			cancel()
			if err != nil {
				return
			}
			continue
		}
		writeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		err := p.conn.Write(writeCtx, websocket.MessageText, payload)
		cancel()
		if err != nil {
			return
		}
	}
}

func (r *room) join(c *websocket.Conn, cancel context.CancelFunc, now time.Time) (*peer, []byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.players) >= capacity {
		return nil, nil, errors.New("体验房已满")
	}
	var raw [12]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return nil, nil, err
	}
	r.serial++
	spawnX := [...]float64{0, .45, -.45, .9, -.9}
	p := &peer{
		player: player{ID: hex.EncodeToString(raw[:]), Name: fmt.Sprintf("静心客%03d", r.serial), X: spawnX[(r.serial-1)%5], Z: 5 + float64((r.serial-1)/5%4)*.45, Action: "idle"},
		conn:   c, cancel: cancel, states: make(chan []byte, 1), errors: make(chan []byte, 4),
		moveAt: now, moveCredit: 1.5, rateAt: now, rateCredit: 40,
	}
	r.players[p.ID] = p
	welcome, _ := json.Marshal(struct {
		Type     string   `json:"type"`
		ID       string   `json:"id"`
		Players  []player `json:"players"`
		Capacity int      `json:"capacity"`
		RoomID   string   `json:"roomId,omitempty"`
	}{"welcome", p.ID, r.snapshotLocked(), capacity, r.id})
	r.broadcastLocked()
	return p, welcome, nil
}

func (r *room) leave(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.players[id]
	if !ok {
		return
	}
	if p.Seat != nil {
		r.seats[*p.Seat] = ""
	}
	delete(r.players, id)
	r.broadcastLocked()
}

func (r *room) snapshotLocked() []player {
	players := make([]player, 0, len(r.players))
	for _, p := range r.players {
		players = append(players, p.player)
	}
	sort.Slice(players, func(i, j int) bool { return players[i].Name < players[j].Name })
	return players
}

func (r *room) broadcastLocked() {
	payload, _ := json.Marshal(struct {
		Type    string   `json:"type"`
		Players []player `json:"players"`
		RoomID  string   `json:"roomId,omitempty"`
	}{"state", r.snapshotLocked(), r.id})
	if r.publish != nil {
		r.publish(r.id, len(r.players), payload)
	}
	for _, p := range r.players {
		select {
		case p.states <- payload:
		default:
			// A slow viewer only needs the latest room snapshot, never an unbounded movement backlog.
			select {
			case <-p.states:
			default:
			}
			select {
			case p.states <- payload:
			default:
			}
		}
	}
}

type command struct {
	Type   string   `json:"type"`
	X      *float64 `json:"x"`
	Z      *float64 `json:"z"`
	Yaw    *float64 `json:"yaw"`
	Action string   `json:"action"`
	Seat   *int     `json:"seat"`
	Hall   string   `json:"hall"`
}

func (r *room) command(id string, data []byte, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.players[id]
	if !ok {
		return errors.New("连接已结束，请重新进入")
	}
	p.rateCredit = math.Min(40, p.rateCredit+math.Max(0, now.Sub(p.rateAt).Seconds())*30)
	p.rateAt = now
	if p.rateCredit < 1 {
		return errors.New("操作过于频繁，请稍后再试")
	}
	p.rateCredit--
	var cmd command
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if len(data) > 1024 || decoder.Decode(&cmd) != nil {
		return errors.New("消息格式不正确")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return errors.New("每条消息只能包含一个操作")
	}
	switch cmd.Type {
	case "move":
		if cmd.X == nil || cmd.Z == nil || cmd.Yaw == nil || !finite(*cmd.X) || !finite(*cmd.Z) || !finite(*cmd.Yaw) || *cmd.X < worldMinX || *cmd.X > worldMaxX || *cmd.Z < worldMinZ || *cmd.Z > worldMaxZ || math.Abs(*cmd.Yaw) > 2*math.Pi {
			return errors.New("移动位置超出院落范围")
		}
		if p.Seat != nil {
			return errors.New("请先起身再走动")
		}
		credit := math.Min(1.5, p.moveCredit+math.Max(0, now.Sub(p.moveAt).Seconds())*5)
		distance := math.Hypot(*cmd.X-p.X, *cmd.Z-p.Z)
		if distance > credit {
			return errors.New("移动过快，请稍后再试")
		}
		p.moveCredit, p.moveAt = credit-distance, now
		p.X, p.Z, p.Yaw = *cmd.X, *cmd.Z, *cmd.Yaw
		p.Action = "walk"
		p.OfferingHall = nil
	case "action":
		switch cmd.Action {
		case "idle", "walk", "greet", "bow":
			if p.Seat != nil && cmd.Action == "walk" {
				return errors.New("请先起身再切换动作")
			}
			if p.Seat != nil && cmd.Action == "idle" {
				cmd.Action = "sit"
			}
		case "sit":
			if p.Seat == nil {
				return errors.New("请先选择空坐垫")
			}
		default:
			return errors.New("不支持这个动作")
		}
		p.Action = cmd.Action
		if cmd.Action != "greet" {
			p.OfferingHall = nil
		}
	case "offer":
		approach, ok := hallApproaches[cmd.Hall]
		if !ok {
			return errors.New("请选择园区内的殿堂")
		}
		if p.Seat != nil {
			return errors.New("请先起身，再到殿堂前礼敬")
		}
		if math.Hypot(p.X-approach[0], p.Z-approach[1]) > 2.5 {
			return errors.New("请先走近殿堂入口")
		}
		hall := cmd.Hall
		p.Action, p.Yaw, p.OfferingHall = "greet", math.Pi, &hall
	case "sit":
		if cmd.Seat == nil || *cmd.Seat < 0 || *cmd.Seat >= len(r.seats) {
			return errors.New("请选择有效坐垫")
		}
		seat := *cmd.Seat
		if r.seats[seat] != "" && r.seats[seat] != id {
			return errors.New("这个坐垫已有人，请选择其他坐垫")
		}
		if p.Seat != nil && *p.Seat != seat {
			return errors.New("请先起身再选择坐垫")
		}
		position := seatPositions[seat]
		if math.Hypot(p.X-position[0], p.Z-position[1]) > 1.2 {
			return errors.New("请先走近坐垫")
		}
		p.Seat, p.X, p.Z, p.Yaw, p.Action = &seat, position[0], position[1], math.Pi, "sit"
		p.OfferingHall = nil
		r.seats[seat] = id
	case "stand":
		if p.Seat != nil {
			r.seats[*p.Seat] = ""
		}
		p.Seat, p.Action = nil, "idle"
		p.OfferingHall = nil
	default:
		return errors.New("不支持这个操作")
	}
	r.broadcastLocked()
	return nil
}

func finite(n float64) bool { return !math.IsNaN(n) && !math.IsInf(n, 0) }

func (r *room) close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range r.players {
		p.cancel()
	}
}

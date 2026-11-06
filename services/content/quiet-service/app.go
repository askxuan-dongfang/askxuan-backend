package main

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
)

const guestCookie = "quiet_guest"

var requestKey = regexp.MustCompile(`^[A-Za-z0-9_-]{8,80}$`)

type app struct {
	store      *storage
	secret     []byte
	owner      string
	origins    []string
	authURL    string
	authClient *http.Client
	mu         sync.Mutex
	rooms      []*room
	pending    map[*room]int
	snapshotMu sync.Mutex
	snapshots  map[string]roomSnapshot
	dirty      map[string]bool
	cancel     context.CancelFunc
	done       chan struct{}
}

func randomHex(bytes int) (string, error) {
	raw := make([]byte, bytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func newApp(store *storage, secret string) (*app, error) {
	owner, err := randomHex(12)
	if err != nil {
		return nil, err
	}
	if secret == "" {
		secret, err = randomHex(32)
		if err != nil {
			return nil, err
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	a := &app{store: store, secret: []byte(secret), owner: owner, pending: map[*room]int{}, snapshots: map[string]roomSnapshot{}, dirty: map[string]bool{}, cancel: cancel, done: make(chan struct{})}
	go a.syncRooms(ctx)
	return a, nil
}

func (a *app) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/quiet-api/session", a.session)
	mux.HandleFunc("/quiet-api/records", a.records)
	mux.HandleFunc("/quiet-api/rooms", a.roomList)
	mux.HandleFunc("/quiet-ws", a.websocket)
	mux.HandleFunc("/health", a.health)
	return mux
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func apiError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"message": message})
}
func tokenHash(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
func (a *app) csrf(token string) string {
	mac := hmac.New(sha256.New, a.secret)
	_, _ = mac.Write([]byte(token))
	return hex.EncodeToString(mac.Sum(nil))
}

func (a *app) guest(req *http.Request) (string, string, error) {
	cookie, err := req.Cookie(guestCookie)
	if err != nil || len(cookie.Value) != 64 {
		return "", "", errors.New("需要静心访客会话")
	}
	if _, err = hex.DecodeString(cookie.Value); err != nil {
		return "", "", errors.New("访客会话无效")
	}
	hash := tokenHash(cookie.Value)
	valid, err := a.store.guest(req.Context(), hash)
	if err != nil {
		return "", "", err
	}
	if !valid {
		return "", "", errors.New("访客会话已失效，请重新进入")
	}
	return hash, cookie.Value, nil
}

func (a *app) session(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		apiError(w, 405, "不支持这个操作")
		return
	}
	if req.Header.Get("Sec-Fetch-Site") == "cross-site" || req.Header.Get("Origin") != "" && !allowedOrigin(req.Header.Get("Origin"), a.origins) {
		apiError(w, 403, "只允许配置的同源入口")
		return
	}
	mode := "guest-local"
	if a.authURL != "" && req.Header.Get("Authorization") != "" {
		if _, failure := a.accountOwner(req); failure != nil {
			failure.write(w)
			return
		}
		mode = "account-local"
	}
	_, token, err := a.guest(req)
	if err != nil {
		if token, err = randomHex(32); err != nil {
			apiError(w, 500, "无法创建访客会话")
			return
		}
		expires := time.Now().Add(30 * 24 * time.Hour)
		if err = a.store.createGuest(req.Context(), tokenHash(token), expires); err != nil {
			apiError(w, 503, "访客会话存储暂不可用")
			return
		}
		http.SetCookie(w, &http.Cookie{Name: guestCookie, Value: token, Path: "/", HttpOnly: true, Secure: req.TLS != nil || req.Header.Get("X-Forwarded-Proto") == "https", SameSite: http.SameSiteStrictMode, MaxAge: 30 * 24 * 60 * 60, Expires: expires})
	}
	writeJSON(w, 200, map[string]any{"csrfToken": a.csrf(token), "persistent": a.store.db != nil, "mode": mode, "accountHistoryAvailable": a.authURL != "", "roomLimit": capacity})
}

func (a *app) records(w http.ResponseWriter, req *http.Request) {
	guest, token, err := a.guest(req)
	if err != nil {
		if req.Header.Get("Authorization") != "" {
			(&accountError{403, 40310, "浏览器访客会话已失效，请重新查看记录。"}).write(w)
			return
		}
		apiError(w, 401, "访客会话已失效，请重新进入")
		return
	}
	if req.Header.Get("Authorization") != "" {
		var failure *accountError
		guest, failure = a.accountOwner(req)
		if failure != nil {
			failure.write(w)
			return
		}
	}
	if req.Method != http.MethodGet {
		if !allowedOrigin(req.Header.Get("Origin"), a.origins) || !hmac.Equal([]byte(req.Header.Get("X-Quiet-CSRF")), []byte(a.csrf(token))) {
			apiError(w, 403, "保存来源或安全校验失败，请重新进入")
			return
		}
	}
	switch req.Method {
	case http.MethodGet:
		records, err := a.store.list(req.Context(), guest)
		if err != nil {
			apiError(w, 503, "访客历史暂不可读取")
			return
		}
		writeJSON(w, 200, records)
	case http.MethodPost:
		if !strings.HasPrefix(req.Header.Get("Content-Type"), "application/json") || !requestKey.MatchString(req.Header.Get("Idempotency-Key")) {
			apiError(w, 400, "保存请求格式异常")
			return
		}
		var record practiceRecord
		decoder := json.NewDecoder(http.MaxBytesReader(w, req.Body, 2048))
		decoder.DisallowUnknownFields()
		var extra any
		if decoder.Decode(&record) != nil || decoder.Decode(&extra) != io.EOF || record.ID != "" {
			apiError(w, 400, "练习记录格式异常")
			return
		}
		stamp, err := time.Parse(time.RFC3339Nano, record.Date)
		if err != nil || stamp.Year() < 2000 || stamp.After(time.Now().Add(5*time.Minute)) || record.Seconds < 0 || record.Seconds > 86400 || record.Hits < 0 || record.Hits > 1000000 || record.Beads < 0 || record.Beads > 1000000 {
			apiError(w, 400, "练习记录数值异常")
			return
		}
		stored, err := a.store.save(req.Context(), guest, req.Header.Get("Idempotency-Key"), record)
		if errors.Is(err, errIdempotency) {
			apiError(w, 409, err.Error())
			return
		}
		if err != nil {
			apiError(w, 503, "记录尚未保存，请保留本地草稿")
			return
		}
		writeJSON(w, 200, stored)
	case http.MethodDelete:
		if err := a.store.clear(req.Context(), guest); err != nil {
			apiError(w, 503, "访客历史尚未清除，请重试")
			return
		}
		writeJSON(w, 200, map[string]bool{"cleared": true})
	default:
		apiError(w, 405, "不支持这个操作")
	}
}

func (a *app) reserve() (*room, func()) {
	a.mu.Lock()
	var selected *room
	for _, r := range a.rooms {
		r.mu.Lock()
		count := len(r.players)
		r.mu.Unlock()
		if count+a.pending[r] < capacity {
			selected = r
			break
		}
	}
	if selected == nil {
		selected = newRoom()
		selected.origins = a.origins
		selected.id = fmt.Sprintf("park-%s-%02d", a.owner[:8], len(a.rooms)+1)
		selected.publish = func(id string, count int, payload []byte) {
			a.snapshotMu.Lock()
			a.snapshots[id] = roomSnapshot{ID: id, Owner: a.owner, Count: count, Snapshot: payload, UpdatedAt: time.Now()}
			a.dirty[id] = true
			a.snapshotMu.Unlock()
		}
		a.rooms = append(a.rooms, selected)
	}
	a.pending[selected]++
	a.mu.Unlock()
	var once sync.Once
	return selected, func() { once.Do(func() { a.mu.Lock(); a.pending[selected]--; a.mu.Unlock() }) }
}

func (a *app) websocket(w http.ResponseWriter, req *http.Request) {
	if _, _, err := a.guest(req); err != nil {
		apiError(w, 401, "请先建立静心访客会话")
		return
	}
	if !allowedOrigin(req.Header.Get("Origin"), a.origins) {
		apiError(w, 403, "只允许配置的同源入口")
		return
	}
	r, release := a.reserve()
	defer release()
	r.serve(w, req, release)
}

func (a *app) syncRooms(ctx context.Context) {
	defer close(a.done)
	flush := time.NewTicker(100 * time.Millisecond)
	defer flush.Stop()
	heartbeat := time.NewTicker(20 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			a.snapshotMu.Lock()
			for id, snapshot := range a.snapshots {
				if snapshot.Count > 0 {
					a.dirty[id] = true
				}
			}
			a.snapshotMu.Unlock()
		case <-flush.C:
			a.snapshotMu.Lock()
			pending := make([]roomSnapshot, 0, len(a.dirty))
			for id := range a.dirty {
				pending = append(pending, a.snapshots[id])
			}
			a.dirty = map[string]bool{}
			a.snapshotMu.Unlock()
			for _, snapshot := range pending {
				snapshot.UpdatedAt = time.Now()
				writeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
				err := a.store.syncRoom(writeCtx, a.owner, snapshot)
				cancel()
				if err != nil {
					a.snapshotMu.Lock()
					a.dirty[snapshot.ID] = true
					a.snapshotMu.Unlock()
				}
			}
		}
	}
}

func (a *app) roomList(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		apiError(w, 405, "不支持这个操作")
		return
	}
	if _, _, err := a.guest(req); err != nil {
		apiError(w, 401, "需要静心访客会话")
		return
	}
	rooms, err := a.store.roomSnapshots(req.Context(), a.owner)
	if err != nil {
		apiError(w, 503, "房间目录暂不可用")
		return
	}
	writeJSON(w, 200, rooms)
}

func (a *app) health(w http.ResponseWriter, req *http.Request) {
	ctx, cancel := context.WithTimeout(req.Context(), time.Second)
	defer cancel()
	if a.store.db != nil {
		if a.store.db.PingContext(ctx) != nil || a.store.redis.Ping(ctx).Err() != nil {
			apiError(w, 503, "静心独立存储未就绪")
			return
		}
	}
	writeJSON(w, 200, map[string]any{"status": "ok", "mode": "guest-local", "persistent": a.store.db != nil, "roomCapacity": capacity})
}

func (a *app) close() {
	a.mu.Lock()
	rooms := append([]*room(nil), a.rooms...)
	a.mu.Unlock()
	for _, r := range rooms {
		r.close()
	}
	a.cancel()
	<-a.done
	if a.store.redis != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		for _, r := range rooms {
			_ = a.store.redis.Del(ctx, a.store.prefix+"room:"+r.id).Err()
		}
	}
}

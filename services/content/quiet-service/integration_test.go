package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func integrationStore(t *testing.T, prefix string) *storage {
	t.Helper()
	if os.Getenv("QUIET_TEST_INTEGRATION") != "1" {
		t.Skip("requires isolated local-stack.sh deps")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s, err := persistentStorage(ctx, "127.0.0.1:3316", os.Getenv("QUIET_MYSQL_PASSWORD"), "127.0.0.1:6386", os.Getenv("QUIET_REDIS_PASSWORD"), prefix)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestMySQLPersistenceGuestIsolationAndRedisRooms(t *testing.T) {
	id, _ := randomHex(8)
	prefix := "quiet-integration:" + id + ":"
	store := integrationStore(t, prefix)
	service, err := newApp(store, "stable-local-test-secret")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(service.handler())
	a, b := newBrowser(t, srv.URL, true), newBrowser(t, srv.URL, true)
	guests := []string{guestHashFor(t, a), guestHashFor(t, b)}
	t.Cleanup(func() {
		cleanup := integrationStore(t, prefix)
		defer cleanup.close()
		for _, guest := range guests {
			_, _ = cleanup.db.Exec(`DELETE FROM quiet_practices WHERE guest_hash=?`, guest)
			_, _ = cleanup.db.Exec(`DELETE FROM quiet_guests WHERE token_hash=?`, guest)
		}
		keys, _ := cleanup.redis.Keys(context.Background(), prefix+"*").Result()
		if len(keys) > 0 {
			_ = cleanup.redis.Del(context.Background(), keys...).Err()
		}
	})
	record := practiceRecord{Date: time.Now().UTC().Truncate(time.Millisecond).Format(time.RFC3339Nano), Seconds: 45, Hits: 4, Beads: 108}
	for i := 0; i < 2; i++ {
		if status, data := a.call(t, "POST", "/quiet-api/records", record, a.csrf, "http://localhost:5186"); status != 200 {
			t.Fatalf("persistent save failed: %d %s", status, data)
		}
	}
	if len(a.list(t)) != 1 || len(b.list(t)) != 0 {
		t.Fatal("persistent guest isolation failed")
	}
	c := a.dial(t)
	welcome := readUntil(t, c, func(e event) bool { return e.Type == "welcome" })
	ctx := context.Background()
	waitUntil(t, func() bool {
		snapshots, err := store.roomSnapshots(ctx, service.owner)
		return err == nil && len(snapshots) == 1 && snapshots[0].ID == welcome.RoomID && snapshots[0].Count == 1 && json.Valid(snapshots[0].Snapshot)
	})
	if ttl := store.redis.TTL(ctx, prefix+"room:"+welcome.RoomID).Val(); ttl <= 0 || ttl > 60*time.Second {
		t.Fatal("room TTL not maintained", ttl)
	}
	if err := store.syncRoom(ctx, "another-owner", roomSnapshot{ID: welcome.RoomID, UpdatedAt: time.Now(), Snapshot: json.RawMessage(`{}`)}); err == nil {
		t.Fatal("room owner could be overwritten")
	}
	_ = c.CloseNow()
	service.close()
	srv.Close()
	store.close()

	store = integrationStore(t, prefix)
	defer store.close()
	service, err = newApp(store, "stable-local-test-secret")
	if err != nil {
		t.Fatal(err)
	}
	defer service.close()
	srv = httptest.NewServer(service.handler())
	defer srv.Close()
	a.base, b.base = srv.URL, srv.URL
	a.session(t, true)
	b.session(t, true)
	if len(a.list(t)) != 1 || len(b.list(t)) != 0 {
		t.Fatal("MySQL record did not survive server/store restart or crossed guest identities")
	}
	if status, _ := b.call(t, "DELETE", "/quiet-api/records", nil, b.csrf, "http://localhost:5186"); status != 200 || len(a.list(t)) != 1 {
		t.Fatal("another guest deleted persisted record")
	}
	if status, _ := a.call(t, "DELETE", "/quiet-api/records", nil, a.csrf, "http://localhost:5186"); status != 200 || len(a.list(t)) != 0 {
		t.Fatal("persistent record deletion failed")
	}
	t.Log("real MySQL reopen persistence, two-cookie guest isolation, delete, idempotency and Redis owner/count/snapshot/TTL verified")
}

func TestDockerTLSWSSChain(t *testing.T) {
	if os.Getenv("QUIET_TEST_WSS") != "1" {
		t.Skip("requires complete isolated Docker/WSS stack")
	}
	certificate, err := os.ReadFile(".local/certs/local.crt")
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(certificate) {
		t.Fatal("invalid local certificate")
	}
	jar, _ := cookiejar.New(nil)
	b := &browser{base: "https://localhost:8446", client: &http.Client{Jar: jar, Timeout: 8 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}}}}
	b.session(t, true)
	parsed, _ := url.Parse(b.base)
	headers := http.Header{"Origin": []string{b.base}}
	for _, cookie := range jar.Cookies(parsed) {
		headers.Add("Cookie", cookie.String())
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "wss://localhost:8446/quiet-ws", &websocket.DialOptions{HTTPClient: b.client, HTTPHeader: headers})
	if err != nil {
		t.Fatal(err)
	}
	defer c.CloseNow()
	welcome := readUntil(t, c, func(e event) bool { return e.Type == "welcome" })
	if welcome.RoomID == "" || welcome.Capacity != 20 {
		t.Fatal("invalid WSS room welcome")
	}
	record := practiceRecord{Date: time.Now().UTC().Truncate(time.Millisecond).Format(time.RFC3339Nano), Seconds: 8, Hits: 2, Beads: 3}
	if status, data := b.call(t, "POST", "/quiet-api/records", record, b.csrf, b.base); status != 200 {
		t.Fatalf("TLS record save failed: %d %s", status, data)
	}
	if status, data := b.call(t, "GET", "/quiet-api/records", nil, "", b.base); status != 200 || !strings.Contains(string(data), `"seconds":8`) {
		t.Fatal("TLS record read failed", status)
	}
	if status, _ := b.call(t, "DELETE", "/quiet-api/records", nil, b.csrf, b.base); status != 200 {
		t.Fatal("TLS record clear failed")
	}
	t.Log("self-signed local certificate verified in test-only trust pool; real nginx HTTPS, HttpOnly guest, WSS and record CRUD chain verified")
}

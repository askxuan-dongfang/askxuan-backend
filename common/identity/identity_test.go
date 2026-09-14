package identity

import (
	"context"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPasswordHashPolicy(t *testing.T) {
	a, e := HashPassword("correct horse battery")
	if e != nil {
		t.Fatal(e)
	}
	b, _ := HashPassword("correct horse battery")
	if a == b || !CheckPassword(a, "correct horse battery") || CheckPassword(a, "different-password") || CheckPassword("123456", "123456") {
		t.Fatal("password hashing broken")
	}
	for _, p := range []string{"short", strings.Repeat("x", 129)} {
		if _, e = HashPassword(p); e == nil {
			t.Fatal("weak password accepted")
		}
	}
	if CheckPassword(strings.Replace(a, "m=19456", "m=999999999", 1), "anything") {
		t.Fatal("unbounded KDF accepted")
	}
}
func TestEmailNormalization(t *testing.T) {
	for _, raw := range []string{"a\r\nBcc: victim@example.org", "Name <a@example.org>", "bad", "a@b"} {
		if _, e := NormalizeEmail(raw); e == nil {
			t.Fatal(raw)
		}
	}
	e, err := NormalizeEmail(" User@Example.org ")
	if err != nil || e != "user@example.org" {
		t.Fatal(e, err)
	}
}
func TestChallengeAndSessionIntegration(t *testing.T) {
	host := os.Getenv("IDENTITY_TEST_REDIS")
	if host == "" {
		t.Skip("set IDENTITY_TEST_REDIS to an isolated test Redis")
	}
	r := redis.MustNewRedis(redis.RedisConf{Host: host, Type: "node"})
	s := Challenges{Redis: r, Secret: RandomID()}
	ctx := context.Background()
	id := RandomID()
	if e := s.Save(ctx, "test", id, "123456", 60); e != nil {
		t.Fatal(e)
	}
	raw, _ := r.GetCtx(ctx, s.Key("test", id))
	if strings.Contains(raw, "123456") {
		t.Fatal("plaintext OTP")
	}
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if s.Consume(ctx, "test", id, "123456", 5) == nil {
				accepted.Add(1)
			}
		}()
	}
	wg.Wait()
	if accepted.Load() != 1 {
		t.Fatalf("replay %d", accepted.Load())
	}
	id = RandomID()
	s.Save(ctx, "test", id, "123456", 60)
	for i := 0; i < 5; i++ {
		if s.Consume(ctx, "test", id, "000000", 5) == nil {
			t.Fatal("bad code")
		}
	}
	if s.Consume(ctx, "test", id, "123456", 5) == nil {
		t.Fatal("attempt limit bypass")
	}
	if e := s.Save(ctx, "test", id, "654321", 60); e != nil {
		t.Fatal(e)
	}
	if e := s.Consume(ctx, "test", id, "654321", 5); e != nil {
		t.Fatal("fresh code inherited old attempt limit", e)
	}
	id = RandomID()
	s.Save(ctx, "test", id, "123456", 1)
	time.Sleep(1100 * time.Millisecond)
	if s.Consume(ctx, "test", id, "123456", 5) == nil {
		t.Fatal("expired code")
	}
	id = RandomID()
	if s.Limit(ctx, "rate", id, 1, 60) != nil || s.Limit(ctx, "rate", id, 1, 60) == nil {
		t.Fatal("rate limit")
	}
	sid, e := CreateSession(ctx, r, "user", 88001, 60)
	if e != nil {
		t.Fatal(e)
	}
	if CheckSession(ctx, r, sid, "user", 88001) != nil {
		t.Fatal("valid session")
	}
	if CheckSession(ctx, r, sid, "admin", 88001) == nil {
		t.Fatal("domain crossover")
	}
	r.IncrCtx(ctx, EpochKey("user", 88001))
	if CheckSession(ctx, r, sid, "user", 88001) == nil {
		t.Fatal("password reset did not revoke")
	}
	sid, _ = CreateSession(ctx, r, "user", 88001, 60)
	RevokeSession(ctx, r, sid)
	if CheckSession(ctx, r, sid, "user", 88001) == nil {
		t.Fatal("logout did not revoke")
	}
}

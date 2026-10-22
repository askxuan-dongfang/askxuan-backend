package identity

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"strconv"
	"strings"
)

// ErrSessionInvalid distinguishes revoked/missing sessions from a Redis outage.
var ErrSessionInvalid = errors.New("会话已失效")

type Session struct {
	Domain string `json:"domain"`
	UserID int64  `json:"userId"`
	Epoch  string `json:"epoch"`
}

func EpochKey(domain string, id int64) string {
	return "identity:epoch:" + domain + ":" + strconv.FormatInt(id, 10)
}
func CreateSession(ctx context.Context, r *redis.Redis, domain string, id int64, ttl int) (string, error) {
	epoch, e := r.GetCtx(ctx, EpochKey(domain, id))
	if e != nil {
		return "", e
	}
	sid := RandomID()
	b, _ := json.Marshal(Session{domain, id, epoch})
	e = r.SetexCtx(ctx, "identity:session:"+sid, string(b), ttl)
	return sid, e
}
func CheckSession(ctx context.Context, r *redis.Redis, sid, domain string, id int64) error {
	if sid == "" {
		return ErrSessionInvalid
	}
	b, e := r.GetCtx(ctx, "identity:session:"+sid)
	if e != nil {
		return e
	}
	var s Session
	if json.Unmarshal([]byte(b), &s) != nil || s.Domain != domain || s.UserID != id {
		return ErrSessionInvalid
	}
	epoch, e := r.GetCtx(ctx, EpochKey(domain, id))
	if e != nil {
		return e
	}
	if epoch != s.Epoch {
		return ErrSessionInvalid
	}
	return nil
}
func SessionDomain(userType string, roles []string) string {
	if userType == "user" {
		return "user"
	}
	return "admin"
}
func RevokeSession(ctx context.Context, r *redis.Redis, sid string) error {
	if sid == "" || strings.Contains(sid, ":") {
		return nil
	}
	_, e := r.DelCtx(ctx, "identity:session:"+sid)
	return e
}

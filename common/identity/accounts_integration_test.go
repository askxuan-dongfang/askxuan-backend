package identity

import (
	"context"
	"fmt"
	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"os"
	"testing"
)

type testMailer struct {
	code string
	fail bool
}

func (m *testMailer) Ready() bool { return true }
func (m *testMailer) Send(_ context.Context, _, code, _ string) error {
	m.code = code
	if m.fail {
		return fmt.Errorf("delivery failed")
	}
	return nil
}

// Opt-in integration uses only a disposable database with the auth migration.
func TestAccountsIntegration(t *testing.T) {
	dsn, host := os.Getenv("IDENTITY_TEST_MYSQL"), os.Getenv("IDENTITY_TEST_REDIS")
	if dsn == "" || host == "" {
		t.Skip("set disposable IDENTITY_TEST_MYSQL and IDENTITY_TEST_REDIS")
	}
	ctx := context.Background()
	db := sqlx.NewMysql(dsn)
	r := redis.MustNewRedis(redis.RedisConf{Host: host, Type: "node"})
	mail := &testMailer{}
	s := Accounts{DB: db, Challenges: &Challenges{Redis: r, Secret: RandomID()}, Mailer: mail}
	name := "qa_" + RandomID()[:20]
	email := name + "@example.test"
	password := "Account-integration-2026!"
	if e := s.SendCode(ctx, "admin", email, "register"); e == nil {
		t.Fatal("privileged registration")
	}
	if e := s.SendCode(ctx, "user", email, "register"); e != nil {
		t.Fatal(e)
	}
	if _, e := s.Register(ctx, email, name, password, mail.code, ""); e == nil {
		t.Fatal("missing agreement")
	}
	id, e := s.Register(ctx, email, name, password, mail.code, AgreementVersion)
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		db.ExecCtx(ctx, "DELETE FROM auth_identity WHERE domain='user' AND user_id=?", id)
		db.ExecCtx(ctx, "DELETE FROM askxuan_user.user_profile WHERE user_id=?", id)
		db.ExecCtx(ctx, "DELETE FROM askxuan_user.user WHERE id=?", id)
	}()
	if _, e = s.Register(ctx, email, name, password, mail.code, AgreementVersion); e == nil {
		t.Fatal("replayed registration")
	}
	a, e := s.PasswordLogin(ctx, "user", email, password)
	if e != nil || a.UserID != id {
		t.Fatal("email login", e)
	}
	if _, e = s.PasswordLogin(ctx, "user", name, password); e != nil {
		t.Fatal("username login", e)
	}
	if _, e = s.PasswordLogin(ctx, "admin", name, password); e == nil {
		t.Fatal("domain crossover")
	}
	if _, e = s.PasswordLogin(ctx, "user", name, "1234"); e == nil {
		t.Fatal("demo password bypass")
	}
	sid, e := s.IssueSession(ctx, "user", id, a.PasswordHash, 60)
	if e != nil {
		t.Fatal(e)
	}
	// Fresh challenge namespace avoids waiting on the preceding email cooldown.
	s.Challenges.Secret = RandomID()
	if e = s.SendCode(ctx, "user", email, "reset"); e != nil {
		t.Fatal(e)
	}
	next := "Changed-integration-2026!"
	if e = s.Reset(ctx, "user", email, next, mail.code); e != nil {
		t.Fatal(e)
	}
	if CheckSession(ctx, r, sid, "user", id) == nil {
		t.Fatal("reset retained old session")
	}
	if _, e = s.IssueSession(ctx, "user", id, a.PasswordHash, 60); e == nil {
		t.Fatal("old credential crossed reset boundary")
	}
	if _, e = s.PasswordLogin(ctx, "user", name, password); e == nil {
		t.Fatal("old password")
	}
	if _, e = s.PasswordLogin(ctx, "user", name, next); e != nil {
		t.Fatal(e)
	}
	if e = s.Reset(ctx, "user", email, password, mail.code); e == nil {
		t.Fatal("reset replay")
	}
	mail.fail = true
	s.Challenges.Secret = RandomID()
	if e = s.SendCode(ctx, "user", email, "reset"); e == nil {
		t.Fatal("SMTP failure reported success")
	}
	if e = s.Reset(ctx, "user", email, password, mail.code); e == nil {
		t.Fatal("undelivered code retained")
	}
}

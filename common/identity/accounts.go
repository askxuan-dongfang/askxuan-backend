package identity

import (
	"context"
	"errors"
	"fmt"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"net/mail"
	"regexp"
	"strings"
)

// Identity queries contain credential hashes and verified contacts. Never log SQL parameters.
func init() { sqlx.DisableLog() }

const AgreementVersion = "2026-09-15"

var usernamePattern = regexp.MustCompile(`^[a-z][a-z0-9_]{3,31}$`)

type Account struct {
	Domain       string `db:"domain"`
	UserID       int64  `db:"user_id"`
	Username     string `db:"username"`
	Email        string `db:"email"`
	PasswordHash string `db:"password_hash"`
}
type Mailer interface {
	Ready() bool
	Send(context.Context, string, string, string) error
}
type Accounts struct {
	DB         sqlx.SqlConn
	Challenges *Challenges
	Mailer     Mailer
}

func NormalizeEmail(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	a, e := mail.ParseAddress(value)
	if e != nil || a.Address != value || len(value) > 254 || !strings.Contains(value, ".") {
		return "", fmt.Errorf("请输入有效邮箱")
	}
	return value, nil
}
func (s *Accounts) Find(ctx context.Context, domain, account string) (*Account, error) {
	if domain != "user" && domain != "admin" {
		return nil, fmt.Errorf("账户类型无效")
	}
	var a Account
	e := s.DB.QueryRowCtx(ctx, &a, `SELECT domain,user_id,username,email,password_hash FROM auth_identity WHERE domain=? AND (username=? OR email=?)`, domain, strings.ToLower(strings.TrimSpace(account)), strings.ToLower(strings.TrimSpace(account)))
	return &a, e
}
func (s *Accounts) SendCode(ctx context.Context, domain, email, purpose string) error {
	email, e := NormalizeEmail(email)
	if e != nil {
		return e
	}
	if domain != "user" && domain != "admin" {
		return fmt.Errorf("账户类型无效")
	}
	if purpose != "register" && purpose != "reset" && purpose != "master_register" && purpose != "temple_register" && purpose != "activate" {
		return fmt.Errorf("验证用途无效")
	}
	if (purpose == "register" && domain != "user") || ((purpose == "master_register" || purpose == "temple_register" || purpose == "activate") && domain != "admin") {
		return fmt.Errorf("工作账户不开放注册")
	}
	if !s.Mailer.Ready() {
		return fmt.Errorf("邮件服务尚未配置")
	}
	if e = s.Challenges.Limit(ctx, "email-minute", email, 1, 60); e != nil {
		return e
	}
	if e = s.Challenges.Limit(ctx, "email-day", email, 10, 86400); e != nil {
		return e
	}
	a, lookup := s.Find(ctx, domain, email)
	if lookup != nil && !errors.Is(lookup, sqlx.ErrNotFound) {
		return fmt.Errorf("账户服务暂不可用")
	}
	// Same response for unknown reset addresses and existing registration addresses.
	if purpose == "reset" && lookup != nil || purpose == "register" && lookup == nil {
		return nil
	}
	if purpose == "reset" && a.Email != email {
		return nil
	}
	if purpose == "master_register" || purpose == "temple_register" {
		if lookup == nil {
			return nil
		}
		var count int64
		if e = s.DB.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM master_invitation WHERE email=?", email); e != nil {
			return e
		}
		if count > 0 {
			return nil
		}
	}
	if purpose == "activate" {
		var count int64
		if e = s.DB.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM master_invitation WHERE email=? AND status='pending'", email); e != nil {
			return e
		}
		if count != 1 {
			return nil
		}
	}
	code := digits(6)
	id := domain + ":" + purpose + ":" + email
	if e = s.Challenges.Save(ctx, "email", id, code, 300); e != nil {
		return fmt.Errorf("验证服务暂不可用")
	}
	label := "注册"
	if purpose == "activate" {
		label = "账号激活"
	}
	if purpose == "reset" {
		label = "重置密码"
	}
	if e = s.Mailer.Send(ctx, email, code, label); e != nil {
		s.Challenges.Redis.DelCtx(ctx, s.Challenges.Key("email", id))
		return fmt.Errorf("邮件发送失败，请稍后重试")
	}
	return nil
}
func (s *Accounts) Register(ctx context.Context, email, username, password, code, agreement string) (int64, error) {
	email, e := NormalizeEmail(email)
	if e != nil {
		return 0, e
	}
	username = strings.ToLower(strings.TrimSpace(username))
	if !usernamePattern.MatchString(username) {
		return 0, fmt.Errorf("用户名需以字母开头，包含 4–32 位小写字母、数字或下划线")
	}
	if agreement != AgreementVersion {
		return 0, fmt.Errorf("请阅读并同意当前用户协议和隐私政策")
	}
	hash, e := HashPassword(password)
	if e != nil {
		return 0, e
	}
	if e = s.Challenges.Consume(ctx, "email", "user:register:"+email, code, 5); e != nil {
		return 0, e
	}
	var id int64
	e = s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		r, e := tx.ExecCtx(ctx, `INSERT INTO askxuan_user.user(mobile,password,nickname,avatar,gender,region,bio,status) VALUES (NULL,'',?,'','unknown','','',1)`, username)
		if e != nil {
			return e
		}
		id, e = r.LastInsertId()
		if e != nil {
			return e
		}
		if _, e = tx.ExecCtx(ctx, `INSERT INTO askxuan_user.user_profile(user_id,preference_tags,total_orders,total_spent,last_active_time) VALUES (?,'',0,0,NOW())`, id); e != nil {
			return e
		}
		_, e = tx.ExecCtx(ctx, `INSERT INTO auth_identity(domain,user_id,username,email,password_hash,agreement_version,verified_at) VALUES ('user',?,?,?,?,?,NOW())`, id, username, email, hash, agreement)
		return e
	})
	if e != nil {
		return 0, fmt.Errorf("注册未完成，邮箱或用户名可能已被使用，请重新获取验证码后重试")
	}
	return id, nil
}
func (s *Accounts) PasswordLogin(ctx context.Context, domain, account, password string) (*Account, error) {
	key := domain + ":" + strings.ToLower(strings.TrimSpace(account))
	if e := s.Challenges.Limit(ctx, "login", key, 10, 600); e != nil {
		return nil, e
	}
	a, e := s.Find(ctx, domain, account)
	if e != nil || !CheckPassword(a.PasswordHash, password) {
		return nil, fmt.Errorf("账号或密码不正确")
	}
	return a, nil
}
func (s *Accounts) Reset(ctx context.Context, domain, email, password, code string) error {
	email, e := NormalizeEmail(email)
	if e != nil {
		return e
	}
	hash, e := HashPassword(password)
	if e != nil {
		return e
	}
	if e = s.Challenges.Consume(ctx, "email", domain+":reset:"+email, code, 5); e != nil {
		return e
	}
	a, e := s.Find(ctx, domain, email)
	if e != nil {
		return fmt.Errorf("验证码无效或已过期")
	}
	// Bump the session epoch before updating the password; fail closed if Redis is unavailable.
	if _, e = s.Challenges.Redis.IncrCtx(ctx, EpochKey(domain, a.UserID)); e != nil {
		return fmt.Errorf("会话服务暂不可用")
	}
	e = s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		if _, err := tx.ExecCtx(ctx, `UPDATE auth_identity SET password_hash=? WHERE domain=? AND user_id=?`, hash, domain, a.UserID); err != nil {
			return err
		}
		table := "askxuan_user.user"
		if domain == "admin" {
			table = "askxuan_auth.admin_account"
		}
		_, err := tx.ExecCtx(ctx, "UPDATE "+table+" SET password=? WHERE id=?", hash, a.UserID)
		return err
	})
	if e != nil {
		return fmt.Errorf("密码重置失败，请重新获取验证码")
	}
	// Catch concurrent login while the database update was in flight.
	if _, e = s.Challenges.Redis.IncrCtx(ctx, EpochKey(domain, a.UserID)); e != nil {
		return fmt.Errorf("密码已更新，会话撤销尚未完成，请联系管理员")
	}
	return nil
}

// IssueSession rechecks the exact credential after session creation. A reset before
// creation changes the hash; a reset after rechecking invalidates the session epoch.
func (s *Accounts) IssueSession(ctx context.Context, domain string, id int64, expectedHash string, ttl int) (string, error) {
	sid, err := CreateSession(ctx, s.Challenges.Redis, domain, id, ttl)
	if err != nil {
		return "", err
	}
	var current string
	err = s.DB.QueryRowCtx(ctx, &current, "SELECT password_hash FROM auth_identity WHERE domain=? AND user_id=?", domain, id)
	if errors.Is(err, sqlx.ErrNotFound) && domain == "admin" {
		err = s.DB.QueryRowCtx(ctx, &current, "SELECT password FROM admin_account WHERE id=?", id)
	}
	if err != nil || current != expectedHash {
		_ = RevokeSession(ctx, s.Challenges.Redis, sid)
		return "", fmt.Errorf("账户凭据已变更，请重新登录")
	}
	return sid, nil
}

package identity

import (
	"context"
	"fmt"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"strings"
)

// Enrollment is operator-only. The existing business ID must be verified out of
// band; possession of an email alone never claims an existing account.
func (s *Accounts) Enroll(ctx context.Context, domain string, id int64, email, username, password, code string, send bool) error {
	if (domain != "user" && domain != "admin") || id <= 0 {
		return fmt.Errorf("账户类型或 ID 无效")
	}
	email, err := NormalizeEmail(email)
	if err != nil {
		return err
	}
	username = strings.ToLower(strings.TrimSpace(username))
	if !usernamePattern.MatchString(username) {
		return fmt.Errorf("用户名需以字母开头，包含 4–32 位字母、数字或下划线")
	}
	table := "askxuan_user.user"
	if domain == "admin" {
		table = "askxuan_auth.admin_account"
	}
	var exists int
	if err = s.DB.QueryRowCtx(ctx, &exists, "SELECT COUNT(*) FROM "+table+" WHERE id=?", id); err != nil || exists != 1 {
		return fmt.Errorf("原账户不存在")
	}
	binding := fmt.Sprintf("%s:%d:%s:%s", domain, id, email, username)
	if send {
		if !s.Mailer.Ready() {
			return fmt.Errorf("邮件服务尚未配置")
		}
		if err = s.Challenges.Limit(ctx, "enroll", email, 1, 60); err != nil {
			return err
		}
		value := digits(6)
		if err = s.Challenges.Save(ctx, "enroll", binding, value, 300); err != nil {
			return err
		}
		if err = s.Mailer.Send(ctx, email, value, "绑定现有账户"); err != nil {
			_, _ = s.Challenges.Redis.DelCtx(ctx, s.Challenges.Key("enroll", binding))
			return fmt.Errorf("邮件发送失败")
		}
		return nil
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	if err = s.Challenges.Consume(ctx, "enroll", binding, code, 5); err != nil {
		return err
	}
	// Only an unverified reserved test placeholder can be upgraded. Verified identities remain immutable here.
	err = s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		result, e := tx.ExecCtx(ctx, `UPDATE auth_identity SET username=?,email=?,password_hash=?,verified_at=NOW() WHERE domain=? AND user_id=? AND verified_at IS NULL AND email LIKE '%.invalid'`, username, email, hash, domain, id)
		if e != nil {
			return e
		}
		n, e := result.RowsAffected()
		if e != nil {
			return e
		}
		if n == 0 {
			if _, e := tx.ExecCtx(ctx, `INSERT INTO auth_identity(domain,user_id,username,email,password_hash,verified_at) VALUES (?,?,?,?,?,NOW())`, domain, id, username, email, hash); e != nil {
				return e
			}
		}
		_, e = tx.ExecCtx(ctx, "UPDATE "+table+" SET password=? WHERE id=?", hash, id)
		return e
	})
	if err != nil {
		return fmt.Errorf("绑定失败，账户、邮箱或用户名可能已经绑定")
	}
	if _, err = s.Challenges.Redis.IncrCtx(ctx, EpochKey(domain, id)); err != nil {
		return fmt.Errorf("绑定已保存，但会话撤销失败，请检查 Redis")
	}
	return nil
}

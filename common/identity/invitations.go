package identity

import (
	"context"
	"fmt"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"strings"
)

type MasterInvitation struct {
	ID         int64  `db:"id" json:"id"`
	AccountID  int64  `db:"account_id" json:"accountId"`
	TempleCode string `db:"temple_code" json:"templeCode"`
	MasterCode string `db:"master_code" json:"masterCode"`
	Email      string `db:"email" json:"email"`
	Username   string `db:"username" json:"username"`
	Status     string `db:"status" json:"status"`
}

const invitationColumns = "id,account_id,temple_code,master_code,email,username,status"

func (s *Accounts) ManagedInvitations(ctx context.Context, temple string) ([]MasterInvitation, error) {
	rows := []MasterInvitation{}
	e := s.DB.QueryRowsCtx(ctx, &rows, "SELECT "+invitationColumns+" FROM master_invitation WHERE temple_code=? ORDER BY id DESC LIMIT 200", temple)
	return rows, e
}
func (s *Accounts) InviteManagedMaster(ctx context.Context, actor int64, temple, master, email, username string) error {
	email, e := NormalizeEmail(email)
	if e != nil {
		return e
	}
	username = strings.ToLower(strings.TrimSpace(username))
	if !usernamePattern.MatchString(username) {
		return fmt.Errorf("用户名需为 4–32 位小写字母、数字或下划线，以字母开头")
	}
	return s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var name string
		if e := tx.QueryRowCtx(ctx, &name, `SELECT m.dharma_name FROM askxuan_master.master m JOIN askxuan_temple.temple t ON t.code=m.temple_code WHERE m.code=? AND m.temple_code=? AND m.manage_by='temple' AND m.auth_status='已认证' AND m.platform_status='normal' AND t.status IN ('正常','推荐') FOR UPDATE`, master, temple); e != nil {
			return fmt.Errorf("仅可为本寺已认证的纳管大师分配账号")
		}
		var n int64
		if e := tx.QueryRowCtx(ctx, &n, "SELECT COUNT(*) FROM admin_account WHERE master_id=?", master); e != nil {
			return e
		}
		if n > 0 {
			return fmt.Errorf("该大师已绑定账号，请管理现有账号")
		}
		if e := tx.QueryRowCtx(ctx, &n, "SELECT COUNT(*) FROM auth_identity WHERE domain='admin' AND (email=? OR username=?)", email, username); e != nil {
			return e
		}
		if n > 0 {
			return fmt.Errorf("邮箱或用户名已有工作账号，请核实后使用其他账号")
		}
		var role int64
		if e := tx.QueryRowCtx(ctx, &role, "SELECT id FROM role WHERE code='master'"); e != nil {
			return e
		}
		r, e := tx.ExecCtx(ctx, "INSERT INTO admin_account(account,password,name,role_id,temple_id,master_id,status) VALUES (?,'',?,?,?,?,'disabled')", username, name, role, temple, master)
		if e != nil {
			return fmt.Errorf("账号已存在，无法分配")
		}
		id, e := r.LastInsertId()
		if e != nil {
			return e
		}
		_, e = tx.ExecCtx(ctx, "INSERT INTO master_invitation(account_id,temple_code,master_code,email,username,created_by) VALUES (?,?,?,?,?,?)", id, temple, master, email, username, actor)
		if e != nil {
			return fmt.Errorf("邮箱或大师已存在分配记录")
		}
		return nil
	})
}
func (s *Accounts) ActivateManagedMaster(ctx context.Context, email, password, code, agreement string) error {
	email, e := NormalizeEmail(email)
	if e != nil {
		return e
	}
	if agreement != AgreementVersion {
		return fmt.Errorf("请同意当前协议")
	}
	hash, e := HashPassword(password)
	if e != nil {
		return e
	}
	if e = s.Challenges.Consume(ctx, "email", "admin:activate:"+email, code, 5); e != nil {
		return e
	}
	return s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var inv MasterInvitation
		if e := tx.QueryRowCtx(ctx, &inv, "SELECT "+invitationColumns+" FROM master_invitation WHERE email=? AND status='pending' FOR UPDATE", email); e != nil {
			return fmt.Errorf("分配记录无效，请联系寺院管理员")
		}
		var n int64
		if e := tx.QueryRowCtx(ctx, &n, `SELECT COUNT(*) FROM askxuan_master.master m JOIN askxuan_temple.temple t ON t.code=m.temple_code WHERE m.code=? AND m.temple_code=? AND m.manage_by='temple' AND m.auth_status='已认证' AND m.platform_status='normal' AND t.status IN ('正常','推荐')`, inv.MasterCode, inv.TempleCode); e != nil {
			return e
		}
		if n != 1 {
			return ErrOnboardingDenied
		}
		if _, e := tx.ExecCtx(ctx, "INSERT INTO auth_identity(domain,user_id,username,email,password_hash,agreement_version,verified_at) VALUES ('admin',?,?,?,?,?,NOW())", inv.AccountID, inv.Username, email, hash, agreement); e != nil {
			return fmt.Errorf("邮箱或用户名已被使用，请联系寺院管理员")
		}
		if _, e := tx.ExecCtx(ctx, "UPDATE admin_account SET password=?,status='enabled' WHERE id=? AND master_id=? AND temple_id=?", hash, inv.AccountID, inv.MasterCode, inv.TempleCode); e != nil {
			return e
		}
		_, e := tx.ExecCtx(ctx, "UPDATE master_invitation SET status='activated' WHERE id=?", inv.ID)
		return e
	})
}
func (s *Accounts) RevokeManagedMaster(ctx context.Context, temple string, id int64) error {
	return s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var inv MasterInvitation
		if e := tx.QueryRowCtx(ctx, &inv, "SELECT "+invitationColumns+" FROM master_invitation WHERE id=? AND temple_code=? FOR UPDATE", id, temple); e != nil {
			return ErrOnboardingDenied
		}
		var n int64
		if e := tx.QueryRowCtx(ctx, &n, "SELECT COUNT(*) FROM admin_account a JOIN askxuan_master.master m ON m.code=a.master_id WHERE a.id=? AND a.temple_id=? AND m.temple_code=? AND m.manage_by='temple'", inv.AccountID, temple, temple); e != nil {
			return e
		}
		if n != 1 {
			return ErrOnboardingDenied
		}
		if _, e := tx.ExecCtx(ctx, "UPDATE admin_account SET status='disabled' WHERE id=?", inv.AccountID); e != nil {
			return e
		}
		if _, e := s.Challenges.Redis.IncrCtx(ctx, EpochKey("admin", inv.AccountID)); e != nil {
			return e
		}
		_, e := tx.ExecCtx(ctx, "UPDATE master_invitation SET status='revoked' WHERE id=?", id)
		return e
	})
}

// RestoreManagedMaster restores the existing assignment; it never changes its
// identity or temple. Accounts not yet activated return to the activation flow.
func (s *Accounts) RestoreManagedMaster(ctx context.Context, temple string, id int64) error {
	return s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var inv MasterInvitation
		if e := tx.QueryRowCtx(ctx, &inv, "SELECT "+invitationColumns+" FROM master_invitation WHERE id=? AND temple_code=? AND status='revoked' FOR UPDATE", id, temple); e != nil {
			return ErrOnboardingDenied
		}
		var n int64
		if e := tx.QueryRowCtx(ctx, &n, `SELECT COUNT(*) FROM admin_account a JOIN askxuan_master.master m ON m.code=a.master_id JOIN askxuan_temple.temple t ON t.code=m.temple_code WHERE a.id=? AND a.temple_id=? AND m.temple_code=? AND m.manage_by='temple' AND m.auth_status='已认证' AND m.platform_status='normal' AND t.status IN ('正常','推荐')`, inv.AccountID, temple, temple); e != nil {
			return e
		}
		if n != 1 {
			return ErrOnboardingDenied
		}
		if e := tx.QueryRowCtx(ctx, &n, "SELECT COUNT(*) FROM auth_identity WHERE domain='admin' AND user_id=? AND email=?", inv.AccountID, inv.Email); e != nil {
			return e
		}
		status, accountStatus := "pending", "disabled"
		if n == 1 {
			status, accountStatus = "activated", "enabled"
		}
		if _, e := s.Challenges.Redis.IncrCtx(ctx, EpochKey("admin", inv.AccountID)); e != nil {
			return e
		}
		if _, e := tx.ExecCtx(ctx, "UPDATE admin_account SET status=? WHERE id=?", accountStatus, inv.AccountID); e != nil {
			return e
		}
		_, e := tx.ExecCtx(ctx, "UPDATE master_invitation SET status=? WHERE id=?", status, id)
		return e
	})
}

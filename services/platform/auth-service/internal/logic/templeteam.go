package logic

import (
	"context"
	"github.com/askxuan/auth-service/internal/svc"
	"github.com/askxuan/common"
	"github.com/askxuan/common/adminpolicy"
	"github.com/askxuan/common/identity"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"regexp"
	"strings"
	"unicode/utf8"
)

type TempleTeamMember struct {
	ID       int64  `db:"id" json:"id"`
	Account  string `db:"account" json:"account"`
	Name     string `db:"name" json:"name"`
	Role     string `db:"role" json:"role"`
	Status   string `db:"status" json:"status"`
	Revision int64  `db:"revision" json:"revision"`
}
type TempleTeamInput struct {
	ID       int64  `json:"id"`
	Revision int64  `json:"revision"`
	Account  string `json:"account"`
	Password string `json:"password"`
	Name     string `json:"name"`
	Role     string `json:"role"`
	Status   string `json:"status"`
}

var teamAccountPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.@+-]{3,63}$`)

func validTeamInput(v TempleTeamInput) bool {
	if v.ID < 0 || v.Revision < 0 || !adminpolicy.IsTempleStaff(v.Role) || (v.Status != "enabled" && v.Status != "disabled") || strings.TrimSpace(v.Name) == "" || utf8.RuneCountInString(v.Name) > 64 {
		return false
	}
	if v.ID == 0 && (!teamAccountPattern.MatchString(v.Account) || v.Password == "") {
		return false
	}
	if v.Password != "" && (len(v.Password) < 12 || len(v.Password) > 128) {
		return false
	}
	return true
}
func TempleTeamList(ctx context.Context, s *svc.ServiceContext, c *common.CustomClaims) ([]TempleTeamMember, error) {
	if c == nil || !c.HasRole("temple_admin") || c.TempleCode == "" {
		return nil, common.ErrForbidden
	}
	rows := []TempleTeamMember{}
	err := s.DB.QueryRowsCtx(ctx, &rows, `SELECT a.id,a.account,a.name,r.code AS role,a.status,a.authz_version AS revision FROM admin_account a JOIN role r ON r.id=a.role_id WHERE a.temple_id=? AND r.code IN ('temple_reception','temple_operations','temple_finance') ORDER BY a.id DESC LIMIT 500`, c.TempleCode)
	return rows, err
}
func SaveTempleTeam(ctx context.Context, s *svc.ServiceContext, c *common.CustomClaims, v TempleTeamInput) (int64, error) {
	if c == nil || !c.HasRole("temple_admin") || c.TempleCode == "" || c.UserId == v.ID {
		return 0, common.ErrForbidden
	}
	if !validTeamInput(v) {
		return 0, common.ErrParam
	}
	password := ""
	var err error
	if v.Password != "" {
		password, err = identity.HashPassword(v.Password)
		if err != nil {
			return 0, common.ErrSystem
		}
	}
	id := v.ID
	err = s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		// Lock and revalidate the owner as well as the target: a parallel rebind or demotion must not write the former tenant.
		var owner struct {
			Role     string `db:"role"`
			Temple   string `db:"temple"`
			Status   string `db:"status"`
			Revision int64  `db:"revision"`
		}
		if e := tx.QueryRowCtx(ctx, &owner, `SELECT r.code AS role,a.temple_id AS temple,a.status,a.authz_version AS revision FROM admin_account a JOIN role r ON r.id=a.role_id WHERE a.id=? FOR UPDATE`, c.UserId); e != nil {
			return e
		}
		if owner.Role != "temple_admin" || owner.Status != "enabled" || owner.Temple != c.TempleCode || owner.Revision != c.AccountVersion {
			return common.ErrForbidden
		}
		var roleID int64
		if e := tx.QueryRowCtx(ctx, &roleID, `SELECT id FROM role WHERE code=? AND authz_scope='temple'`, v.Role); e != nil {
			return e
		}
		if id == 0 {
			var n int64
			if e := tx.QueryRowCtx(ctx, &n, `SELECT (SELECT COUNT(*) FROM admin_account WHERE account=?) + (SELECT COUNT(*) FROM auth_identity WHERE domain='admin' AND (username=? OR email=?))`, v.Account, v.Account, v.Account); e != nil {
				return e
			}
			if n > 0 {
				return common.ErrUserAlreadyExists
			}
			res, e := tx.ExecCtx(ctx, `INSERT INTO admin_account(account,password,name,role_id,temple_id,master_id,shop_id,status) VALUES(?,?,?,?,?,'',0,?)`, v.Account, password, strings.TrimSpace(v.Name), roleID, c.TempleCode, v.Status)
			if e != nil {
				return e
			}
			id, e = res.LastInsertId()
			if e != nil {
				return e
			}
			_, e = tx.ExecCtx(ctx, `INSERT INTO askxuan_temple.temple_admin(temple_code,account_id,role) VALUES(?,?,'admin')`, c.TempleCode, id)
			return e
		}
		var old struct {
			Role     string `db:"role"`
			Temple   string `db:"temple"`
			Revision int64  `db:"revision"`
		}
		if e := tx.QueryRowCtx(ctx, &old, `SELECT r.code AS role,a.temple_id AS temple,a.authz_version AS revision FROM admin_account a JOIN role r ON r.id=a.role_id WHERE a.id=? FOR UPDATE`, id); e != nil {
			if e == sqlx.ErrNotFound {
				return common.ErrForbidden
			}
			return e
		}
		if old.Temple != c.TempleCode || !adminpolicy.IsTempleStaff(old.Role) {
			return common.ErrForbidden
		}
		if old.Revision != v.Revision {
			return &common.BizError{Code: 40920, Msg: "账号已被修改，请刷新后重试"}
		}
		query := `UPDATE admin_account SET name=?,role_id=?,status=?,authz_version=authz_version+1`
		args := []any{strings.TrimSpace(v.Name), roleID, v.Status}
		if password != "" {
			query += `,password=?`
			args = append(args, password)
		}
		query += ` WHERE id=?`
		args = append(args, id)
		_, e := tx.ExecCtx(ctx, query, args...)
		if e == nil && password != "" {
			// Verified identity login must use the same new credential as account login.
			_, e = tx.ExecCtx(ctx, `UPDATE auth_identity SET password_hash=? WHERE domain='admin' AND user_id=?`, password, id)
		}
		return e
	})
	return id, err
}

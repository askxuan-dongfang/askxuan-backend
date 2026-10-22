package logic

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/askxuan/auth-service/internal/model"
	"github.com/askxuan/auth-service/internal/svc"
	"github.com/askxuan/common"
	"github.com/askxuan/common/adminpolicy"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var ErrAuthorizationConflict = common.NewBizError(40916, "权限已被更新，请重新加载后提交")
var ErrLastSuper = common.NewBizError(40917, "至少保留一个启用的超级管理员")

type RolePermissions struct {
	RoleID          int64    `json:"roleId"`
	Revision        int64    `json:"revision"`
	PermissionCodes []string `json:"permissionCodes"`
	Editable        bool     `json:"editable"`
}

func GetRolePermissions(ctx context.Context, s *svc.ServiceContext, id int64) (*RolePermissions, error) {
	r, err := s.RoleModel.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrParam
		}
		return nil, common.ErrSystem
	}
	out := &RolePermissions{RoleID: id, Revision: r.AuthzVersion, PermissionCodes: []string{}, Editable: !adminpolicy.Builtin(r.Code) && effectiveRoleScope(r) == "platform"}
	if !out.Editable {
		return out, nil
	}
	snapshot, err := authorizationSnapshot(ctx, s, &model.AdminAccount{}, r)
	if err != nil {
		return nil, err
	}
	out.PermissionCodes = snapshot.Permissions
	return out, nil
}
func ValidatePermissionCodes(codes []string) ([]string, error) {
	if len(codes) > len(adminpolicy.Catalog) {
		return nil, common.ErrParam
	}
	seen := map[string]bool{}
	out := []string{}
	for _, code := range codes {
		if !adminpolicy.Assignable(code) || seen[code] {
			return nil, common.ErrParam
		}
		seen[code] = true
		out = append(out, code)
	}
	for _, code := range out {
		if strings.HasSuffix(code, ".write") && !seen[strings.TrimSuffix(code, ".write")+".read"] {
			return nil, common.ErrParam
		}
	}
	sort.Strings(out)
	return out, nil
}
func SetRolePermissions(ctx context.Context, s *svc.ServiceContext, id, revision int64, codes []string) (*RolePermissions, error) {
	codes, err := ValidatePermissionCodes(codes)
	if err != nil {
		return nil, err
	}
	if revision < 0 {
		return nil, common.ErrParam
	}
	err = s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var role model.Role
		if e := tx.QueryRowCtx(ctx, &role, `SELECT id,name,code,description,authz_scope,authz_version,create_time FROM role WHERE id=? FOR UPDATE`, id); e != nil {
			if errors.Is(e, sqlx.ErrNotFound) {
				return common.ErrParam
			}
			return common.ErrSystem
		}
		if adminpolicy.Builtin(role.Code) || effectiveRoleScope(&role) != "platform" {
			return common.ErrRoleForbidden
		}
		if role.AuthzVersion != revision {
			return ErrAuthorizationConflict
		}
		res, e := tx.ExecCtx(ctx, `UPDATE role SET authz_version=authz_version+1 WHERE id=? AND authz_version=?`, id, revision)
		if e != nil {
			return e
		}
		n, e := res.RowsAffected()
		if e != nil {
			return e
		}
		if n != 1 {
			return ErrAuthorizationConflict
		}
		// Legacy provider permission rows are retained but can never authorize platform APIs.
		for _, cap := range adminpolicy.Catalog {
			if !cap.Assignable {
				continue
			}
			if _, e = tx.ExecCtx(ctx, `DELETE rp FROM role_permission rp JOIN permission p ON p.id=rp.permission_id WHERE rp.role_id=? AND p.code=?`, id, cap.Code); e != nil {
				return e
			}
		}
		for _, code := range codes {
			res, e := tx.ExecCtx(ctx, `INSERT INTO role_permission(role_id,permission_id) SELECT ?,id FROM permission WHERE code=?`, id, code)
			if e != nil {
				return e
			}
			n, e := res.RowsAffected()
			if e != nil {
				return e
			}
			if n != 1 {
				return common.ErrSystem
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &RolePermissions{RoleID: id, Revision: revision + 1, PermissionCodes: codes, Editable: true}, nil
}

// Lock the stable super-role row before checking the last active super. Every
// account mutation below uses this order, preventing concurrent demotions.
func guardLastSuper(ctx context.Context, tx sqlx.Session, id, newRoleID int64, newStatus string) error {
	var superID int64
	if err := tx.QueryRowCtx(ctx, &superID, `SELECT id FROM role WHERE code='platform_super' FOR UPDATE`); err != nil {
		return err
	}
	var current struct {
		RoleID int64  `db:"role_id"`
		Status string `db:"status"`
	}
	if err := tx.QueryRowCtx(ctx, &current, `SELECT role_id,status FROM admin_account WHERE id=? FOR UPDATE`, id); err != nil {
		return err
	}
	if current.RoleID == superID && current.Status == model.AccountStatusEnabled && (newRoleID != superID || newStatus != model.AccountStatusEnabled) {
		var count int64
		if err := tx.QueryRowCtx(ctx, &count, `SELECT COUNT(*) FROM admin_account WHERE role_id=? AND status='enabled'`, superID); err != nil {
			return err
		}
		if count <= 1 {
			return ErrLastSuper
		}
	}
	return nil
}
func updateAccountStatus(ctx context.Context, s *svc.ServiceContext, a *model.AdminAccount, status string) error {
	return s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		// Managed activation/revocation uses invitation -> account lock order.
		// Recheck inside this transaction so a concurrent revoke cannot be undone.
		if status == model.AccountStatusEnabled {
			var invitationStatus string
			err := tx.QueryRowCtx(ctx, &invitationStatus, `SELECT status FROM master_invitation WHERE account_id=? FOR UPDATE`, a.Id)
			if err != nil && !errors.Is(err, sqlx.ErrNotFound) {
				return err
			}
			if err == nil && (invitationStatus == "pending" || invitationStatus == "revoked") {
				return common.NewBizError(40003, "请先完成本人激活或由所属寺院恢复账号分配")
			}
		}
		if err := guardLastSuper(ctx, tx, a.Id, a.RoleId, status); err != nil {
			return err
		}
		_, err := tx.ExecCtx(ctx, `UPDATE admin_account SET status=?,authz_version=authz_version+1 WHERE id=?`, status, a.Id)
		return err
	})
}

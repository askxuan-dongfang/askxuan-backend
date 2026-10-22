package model

import (
	"context"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func initialTempleAuth(status string) string {
	if status == TempleStatusNormal || status == TempleStatusRecommend {
		return "certified"
	}
	if status == TempleStatusBanned {
		return "legacy_unknown"
	}
	return "pending"
}
func initialTemplePlatform(status string) string {
	if status == TempleStatusBanned {
		return "banned"
	}
	return "normal"
}
func IsTemplePublic(t *Temple) bool {
	return t != nil && t.AuthStatus == "certified" && t.PlatformStatus == "normal" && t.ShelfStatus == "on_shelf"
}
func templeLegacyStatus(t *Temple) string {
	if t.PlatformStatus == "banned" {
		return TempleStatusBanned
	}
	if t.AuthStatus != "certified" {
		return TempleStatusPending
	}
	if t.Recommended {
		return TempleStatusRecommend
	}
	return TempleStatusNormal
}

type TempleStatePatch struct {
	Version        int64   `json:"version"`
	PlatformStatus *string `json:"platformStatus,optional"`
	ShelfStatus    *string `json:"shelfStatus,optional"`
	Recommended    *bool   `json:"recommended,optional"`
}

// State changes lock the subject and revoke affected work sessions in one SQL transaction.
// Re-enabling never restores the previous display state or any old session.
func ChangeTempleState(ctx context.Context, db sqlx.SqlConn, code string, patch TempleStatePatch) (*Temple, error) {
	var updated Temple
	err := db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		if err := tx.QueryRowPartialCtx(ctx, &updated, "SELECT id,code,status,auth_status,platform_status,shelf_status,recommended,state_version FROM temple WHERE code=? FOR UPDATE", code); err != nil {
			return err
		}
		if patch.Version != updated.StateVersion {
			return common.NewBizError(40901, "状态已变化，请刷新后重试")
		}
		return applyTempleState(ctx, tx, &updated, patch)
	})
	return &updated, err
}
func applyTempleState(ctx context.Context, tx sqlx.Session, t *Temple, p TempleStatePatch) error {
	if p.PlatformStatus == nil && p.ShelfStatus == nil && p.Recommended == nil {
		return common.ErrParamInvalid
	}
	previousPlatform := t.PlatformStatus
	previousShelf, previousRecommended := t.ShelfStatus, t.Recommended
	if p.PlatformStatus != nil {
		if *p.PlatformStatus != "normal" && *p.PlatformStatus != "banned" {
			return common.ErrParamInvalid
		}
		t.PlatformStatus = *p.PlatformStatus
	}
	if p.ShelfStatus != nil {
		if *p.ShelfStatus != "on_shelf" && *p.ShelfStatus != "off_shelf" {
			return common.ErrParamInvalid
		}
		t.ShelfStatus = *p.ShelfStatus
	}
	if p.Recommended != nil {
		t.Recommended = *p.Recommended
	}
	// Ban always stops new business; restoring access does not silently resume discovery.
	if t.PlatformStatus == "banned" {
		t.ShelfStatus = "off_shelf"
	}
	if t.ShelfStatus == "on_shelf" && t.AuthStatus != "certified" {
		return common.ErrStatusInvalid
	}
	if previousPlatform == t.PlatformStatus && previousShelf == t.ShelfStatus && previousRecommended == t.Recommended {
		return nil
	}
	t.Status = templeLegacyStatus(t)
	if _, err := tx.ExecCtx(ctx, "UPDATE temple SET status=?,platform_status=?,shelf_status=?,recommended=?,state_version=state_version+1 WHERE id=?", t.Status, t.PlatformStatus, t.ShelfStatus, t.Recommended, t.Id); err != nil {
		return err
	}
	if previousPlatform != t.PlatformStatus {
		if _, err := tx.ExecCtx(ctx, "UPDATE askxuan_auth.admin_account SET authz_version=authz_version+1 WHERE temple_id=?", t.Code); err != nil {
			return err
		}
	}
	t.StateVersion++
	return nil
}
func (m *defaultTempleModel) updateLegacyState(ctx context.Context, id int64, status string) error {
	return m.conn.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var t Temple
		if err := tx.QueryRowPartialCtx(ctx, &t, "SELECT id,code,status,auth_status,platform_status,shelf_status,recommended,state_version FROM temple WHERE id=? FOR UPDATE", id); err != nil {
			return err
		}
		var patch TempleStatePatch
		normal, banned := "normal", "banned"
		yes, no := true, false
		switch status {
		case TempleStatusBanned:
			patch.PlatformStatus = &banned
		case TempleStatusNormal:
			patch.PlatformStatus = &normal
			patch.Recommended = &no
		case TempleStatusRecommend:
			patch.PlatformStatus = &normal
			patch.Recommended = &yes
		default:
			return common.ErrParamInvalid
		}
		if status != TempleStatusBanned && t.AuthStatus != "certified" {
			return common.ErrStatusInvalid
		}
		return applyTempleState(ctx, tx, &t, patch)
	})
}

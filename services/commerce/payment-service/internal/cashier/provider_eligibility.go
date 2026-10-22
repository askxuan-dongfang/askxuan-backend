package cashier

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ProviderAllowsExistingPayment is shared by the cashier and legacy internal
// payment entry points. The caller owns any transaction; lock=true holds
// provider rows until the same transaction commits.
func ProviderAllowsExistingPayment(ctx context.Context, q querySession, r OrderRequest, lock bool) (bool, error) {
	return providerAllowsExistingPayment(ctx, q, r, lock)
}

// Existing orders retain their immutable price and remain payable after a
// display-only shelf change. A platform ban, missing subject or unverified
// certification stops a new charge. These checks do not alter fulfillment.
func providerAllowsExistingPayment(ctx context.Context, q querySession, r OrderRequest, lock bool) (bool, error) {
	switch r.OrderType {
	case "booking":
		var subject struct {
			Master string `db:"master_code"`
			Temple string `db:"temple_code"`
		}
		if err := q.QueryRowCtx(ctx, &subject, `SELECT master_code,temple_code FROM askxuan_booking.booking WHERE booking_no=?`, r.OrderNo); err != nil {
			return false, err
		}
		if subject.Temple != "" {
			allowed, err := templeAllowsExistingPayment(ctx, q, subject.Temple, lock)
			if err != nil || !allowed {
				return allowed, err
			}
		}
		if subject.Master != "" {
			return masterAllowsExistingPayment(ctx, q, subject.Master, lock)
		}
		return subject.Temple != "", nil
	case "consultation":
		var subject struct {
			Master string `db:"master_code"`
			Temple string `db:"temple_code"`
		}
		if err := q.QueryRowCtx(ctx, &subject, `SELECT master_code,temple_code FROM askxuan_booking.consultation_order WHERE order_no=?`, r.OrderNo); err != nil {
			return false, err
		}
		if subject.Temple != "" {
			allowed, err := templeAllowsExistingPayment(ctx, q, subject.Temple, lock)
			if err != nil || !allowed {
				return allowed, err
			}
		}
		return masterAllowsExistingPayment(ctx, q, subject.Master, lock)
	case "diy_order":
		var detail struct {
			Fee      float64 `db:"bless_fee"`
			Snapshot string  `db:"pricing_snapshot"`
		}
		if err := q.QueryRowCtx(ctx, &detail, `SELECT bless_fee,COALESCE(pricing_snapshot,'') pricing_snapshot FROM askxuan_diy.diy_order WHERE order_no=?`, r.OrderNo); err != nil {
			return false, err
		}
		var snapshot struct {
			Consecration *struct {
				Provider string `json:"providerType"`
				Temple   string `json:"templeCode"`
				Master   string `json:"masterCode"`
			} `json:"consecration"`
		}
		err := json.Unmarshal([]byte(detail.Snapshot), &snapshot)
		if snapshot.Consecration == nil {
			return detail.Fee == 0, nil
		}
		if err != nil {
			return false, nil
		}
		switch snapshot.Consecration.Provider {
		case "temple":
			return templeAllowsExistingPayment(ctx, q, snapshot.Consecration.Temple, lock)
		case "master":
			return masterAllowsExistingPayment(ctx, q, snapshot.Consecration.Master, lock)
		default:
			return false, nil
		}
	}
	return true, nil
}

func templeAllowsExistingPayment(ctx context.Context, q querySession, code string, lock bool) (bool, error) {
	if code == "" {
		return false, nil
	}
	var state struct {
		Auth     string `db:"auth_status"`
		Platform string `db:"platform_status"`
	}
	query := `SELECT auth_status,platform_status FROM askxuan_temple.temple WHERE code=?`
	if lock {
		query += " FOR SHARE"
	}
	err := q.QueryRowCtx(ctx, &state, query, code)
	if errors.Is(err, sqlx.ErrNotFound) {
		return false, nil
	}
	return state.Auth == "certified" && state.Platform == "normal", err
}
func masterAllowsExistingPayment(ctx context.Context, q querySession, code string, lock bool) (bool, error) {
	if code == "" {
		return false, nil
	}
	var state struct {
		Auth     string `db:"auth_status"`
		Platform string `db:"platform_status"`
		Temple   string `db:"temple_code"`
	}
	query := `SELECT auth_status,platform_status,temple_code FROM askxuan_master.master WHERE code=?`
	if lock {
		query += " FOR SHARE"
	}
	err := q.QueryRowCtx(ctx, &state, query, code)
	if errors.Is(err, sqlx.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if state.Auth != "已认证" || state.Platform != "normal" {
		return false, nil
	}
	if state.Temple != "" {
		return templeAllowsExistingPayment(ctx, q, state.Temple, lock)
	}
	return true, nil
}

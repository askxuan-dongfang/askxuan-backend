package growth

import (
	"context"
	"errors"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"math"
)

// FullRefundAmount translates an explicitly whole original checkout into the
// immutable net cash payment. Partial/unspecified requests are never reinterpreted.
func (s *Store) FullRefundAmount(ctx context.Context, paymentNo, kind, no string, original float64) (float64, error) {
	if math.IsNaN(original) || math.IsInf(original, 0) || original <= 0 {
		return 0, common.ErrParam
	}
	var v struct {
		Gross int64 `db:"gross_cents"`
		Net   int64 `db:"paid_cents"`
	}
	err := s.DB.QueryRowCtx(ctx, &v, `SELECT q.gross_cents,q.payable_cents paid_cents FROM growth_quote q JOIN payment p ON p.payment_no=q.payment_no WHERE p.payment_no=? AND p.order_type=? AND p.order_no=? AND q.order_type=p.order_type AND q.order_no=p.order_no AND q.status IN ('paid','refunded')`, paymentNo, kind, no)
	if errors.Is(err, sqlx.ErrNotFound) {
		return original, nil
	}
	if err != nil {
		return 0, err
	}
	if math.Abs(original*100-float64(v.Gross)) > 0.001 {
		return 0, ErrChanged
	}
	return float64(v.Net) / 100, nil
}

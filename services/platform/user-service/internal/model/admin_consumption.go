package model

import (
	"context"
	"errors"
	"fmt"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"strconv"
	"strings"
)

// A paid business order is counted once, including its refund history. Spending
// is the cash amount actually retained. Recharges, points, mock/demo payments
// and experience orders are not consumption. Database failures are not zeroes.
const UserConsumptionScope = "real_paid_net_refunds"

type UserConsumption struct {
	UserID      int64 `db:"user_id"`
	TotalOrders int   `db:"total_orders"`
	SpentCents  int64 `db:"spent_cents"`
}

func ReadUserConsumption(ctx context.Context, db sqlx.SqlConn, ids []int64) (map[int64]UserConsumption, error) {
	out := make(map[int64]UserConsumption, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		if id <= 0 {
			return nil, fmt.Errorf("invalid user id")
		}
		placeholders[i] = "?"
		args[i] = strconv.FormatInt(id, 10)
		out[id] = UserConsumption{UserID: id}
	}
	// Aggregate successful refunds once per payment before joining. This avoids
	// multiplying payments with multiple refund records or counting retry rows as
	// extra business orders. Integer cents are used throughout the calculation.
	query := `SELECT CAST(p.user_id AS SIGNED) user_id,
 COUNT(DISTINCT p.order_type,p.order_no) total_orders,
 CAST(COALESCE(SUM(CASE WHEN p.status='refunded' THEN 0 ELSE
 GREATEST(0,CAST(ROUND(p.amount*100) AS SIGNED)-COALESCE(r.refund_cents,0)) END),0) AS SIGNED) spent_cents
 FROM askxuan_payment.payment p
 LEFT JOIN (SELECT payment_id,SUM(CAST(ROUND(amount*100) AS SIGNED)) refund_cents
 FROM askxuan_payment.refund WHERE status='success' GROUP BY payment_id) r ON r.payment_id=p.id
 WHERE p.user_id IN (` + strings.Join(placeholders, ",") + `)
 AND p.order_type IN ('booking','consultation','shop_order','diy_order','ai_report')
 AND p.order_no<>'' AND p.order_no NOT LIKE 'EXO-%'
 AND p.channel IN ('wechat','alipay','balance')
 AND p.status IN ('success','refunding','refunded') AND p.amount>0
 GROUP BY p.user_id`
	rows := []UserConsumption{}
	if err := db.QueryRowsCtx(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.UserID] = row
	}
	return out, nil
}

type UserCredential struct {
	Email            string `db:"email"`
	VerifiedAt       string `db:"verified_at"`
	EmailVerified    bool
	CredentialState  string
	CanResetPassword bool
}

func ReadUserCredential(ctx context.Context, db sqlx.SqlConn, id int64) (UserCredential, error) {
	var c UserCredential
	var row struct {
		Email      string `db:"email"`
		VerifiedAt string `db:"verified_at"`
	}
	err := db.QueryRowCtx(ctx, &row, `SELECT email,COALESCE(DATE_FORMAT(verified_at,'%Y-%m-%d %H:%i:%s'),'') verified_at FROM askxuan_auth.auth_identity WHERE domain='user' AND user_id=?`, id)
	if errors.Is(err, sqlx.ErrNotFound) {
		c.CredentialState = "legacy_password"
		return c, nil
	}
	if err != nil {
		return c, err
	}
	c.Email = row.Email
	c.VerifiedAt = row.VerifiedAt
	c.EmailVerified = c.VerifiedAt != "" && !strings.HasSuffix(strings.ToLower(c.Email), ".invalid")
	c.CanResetPassword = c.EmailVerified
	c.CredentialState = "unavailable"
	if c.EmailVerified {
		c.CredentialState = "verified_email"
	}
	return c, nil
}

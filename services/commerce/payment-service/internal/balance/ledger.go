// Package balance owns cash balances. Points and mock payments never fund it.
package balance

import (
	"context"
	"errors"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"math"
	"strconv"
	"strings"
)

var ErrFunds = common.NewBizError(40901, "钱包余额不足，请先充值")
var ErrConflict = common.NewBizError(40902, "订单状态或金额已变化，请刷新后重试")
var ErrUnavailable = common.NewBizError(40903, "该支付渠道暂未开通")

const MaxCents int64 = 100000000 // bounded arithmetic; single payment <= RMB 1m

type Account struct {
	Available int64 `db:"available_cents" json:"availableCents"`
	Held      int64 `db:"held_cents" json:"heldCents"`
}
type Entry struct {
	ID        int64  `db:"id" json:"id"`
	Kind      string `db:"kind" json:"kind"`
	Ref       string `db:"reference_no" json:"referenceNo"`
	Delta     int64  `db:"available_delta" json:"deltaCents"`
	HeldDelta int64  `db:"held_delta" json:"heldDeltaCents"`
	After     int64  `db:"available_after" json:"balanceAfterCents"`
	Created   string `db:"created_at" json:"createdAt"`
}

// Cents rejects fractional cents instead of silently changing the authorized price.
func Cents(yuan float64) (int64, error) {
	if math.IsNaN(yuan) || math.IsInf(yuan, 0) || yuan <= 0 || yuan > float64(MaxCents)/100 {
		return 0, common.ErrParam
	}
	x := math.Round(yuan * 100)
	if math.Abs(yuan*100-x) > 0.000001 {
		return 0, common.ErrParam
	}
	return int64(x), nil
}
func ParseCents(s string) (int64, error) {
	parts := strings.Split(s, ".")
	if len(parts) > 2 || len(parts[0]) == 0 {
		return 0, common.ErrParam
	}
	for _, c := range s {
		if c != '.' && (c < '0' || c > '9') {
			return 0, common.ErrParam
		}
	}
	fraction := "00"
	if len(parts) == 2 {
		if len(parts[1]) < 1 || len(parts[1]) > 2 {
			return 0, common.ErrParam
		}
		fraction = (parts[1] + "0")[:2]
	}
	whole, e := strconv.ParseInt(parts[0], 10, 64)
	if e != nil || whole > MaxCents/100 {
		return 0, common.ErrParam
	}
	f, _ := strconv.ParseInt(fraction, 10, 64)
	n := whole*100 + f
	if n <= 0 || n > MaxCents {
		return 0, common.ErrParam
	}
	return n, nil
}
func Read(ctx context.Context, db sqlx.SqlConn, user string) (Account, error) {
	var a Account
	e := db.QueryRowCtx(ctx, &a, `SELECT available_cents,held_cents FROM wallet_account WHERE user_id=?`, user)
	if errors.Is(e, sqlx.ErrNotFound) {
		e = nil
	}
	return a, e
}

// Change requires the caller's transaction. Idempotency and balance serialize on
// the account row, including changes initiated by different orders/channels.
func Change(ctx context.Context, tx sqlx.Session, user, key, kind, ref string, delta, held int64) error {
	if user == "" || key == "" || delta > MaxCents || delta < -MaxCents || held > MaxCents || held < -MaxCents {
		return common.ErrParam
	}
	if _, e := tx.ExecCtx(ctx, `INSERT INTO wallet_account(user_id) VALUES(?) ON DUPLICATE KEY UPDATE user_id=user_id`, user); e != nil {
		return e
	}
	var a Account
	if e := tx.QueryRowCtx(ctx, &a, `SELECT available_cents,held_cents FROM wallet_account WHERE user_id=? FOR UPDATE`, user); e != nil {
		return e
	}
	var old struct {
		User  string `db:"user_id"`
		Delta int64  `db:"available_delta"`
		Held  int64  `db:"held_delta"`
		Kind  string `db:"kind"`
		Ref   string `db:"reference_no"`
	}
	e := tx.QueryRowCtx(ctx, &old, `SELECT user_id,available_delta,held_delta,kind,reference_no FROM wallet_ledger WHERE event_key=?`, key)
	if e == nil {
		if old.User != user || old.Delta != delta || old.Held != held || old.Kind != kind || old.Ref != ref {
			return ErrConflict
		}
		return nil
	}
	if !errors.Is(e, sqlx.ErrNotFound) {
		return e
	}
	if a.Available+delta < 0 || a.Held+held < 0 {
		return ErrFunds
	}
	if a.Available > math.MaxInt64-MaxCents || a.Held > math.MaxInt64-MaxCents {
		return ErrConflict
	}
	if _, e = tx.ExecCtx(ctx, `UPDATE wallet_account SET available_cents=available_cents+?,held_cents=held_cents+? WHERE user_id=?`, delta, held, user); e != nil {
		return e
	}
	_, e = tx.ExecCtx(ctx, `INSERT INTO wallet_ledger(user_id,event_key,kind,reference_no,available_delta,held_delta,available_after,held_after) VALUES(?,?,?,?,?,?,?,?)`, user, key, kind, ref, delta, held, a.Available+delta, a.Held+held)
	return e
}

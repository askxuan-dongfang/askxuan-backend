package model

import (
	"context"
	"fmt"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"regexp"
)

type demoLedgerKey struct{}

func DemoLedgerContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, demoLedgerKey{}, true)
}
func isDemoLedger(ctx context.Context) bool { v, _ := ctx.Value(demoLedgerKey{}).(bool); return v }

var financeTableNames = regexp.MustCompile(`\b(finance_transaction|finance_ledger_entry|finance_log|settlement|withdrawal)\b`)

// Only trusted SQL identifiers are rewritten; parameters are never transformed.
func financeQuery(ctx context.Context, q string) string {
	if !isDemoLedger(ctx) {
		return q
	}
	return financeTableNames.ReplaceAllString(q, "demo_$1")
}
func paymentLedgerContext(ctx context.Context, kind, no string) (context.Context, error) {
	var channel string
	if e := db.QueryRowCtx(ctx, &channel, `SELECT channel FROM askxuan_payment.payment WHERE order_type=? AND order_no=? ORDER BY id DESC LIMIT 1`, kind, no); e != nil {
		return ctx, e
	}
	if channel == "demo_balance" {
		return DemoLedgerContext(ctx), nil
	}
	return ctx, nil
}

// SimulatePayout never invokes a bank. Ownership, fulfillment and repeat requests
// are checked inside the same transaction as its isolated accounting entries.
func SimulatePayout(ctx context.Context, kind, target string, id int64) error {
	if (kind != "master" && kind != "temple") || target == "" || target == "0" || id <= 0 {
		return common.ErrParam
	}
	code := target
	if kind == "master" {
		if e := db.QueryRowCtx(ctx, &code, `SELECT code FROM askxuan_master.master WHERE id=?`, target); e != nil {
			return e
		}
	}
	return db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var s struct {
			Type   string  `db:"source_type"`
			No     string  `db:"source_no"`
			Status string  `db:"status"`
			Amount float64 `db:"settle_amount"`
		}
		if e := tx.QueryRowCtx(ctx, &s, `SELECT source_type,source_no,status,settle_amount FROM demo_settlement WHERE id=? AND settle_type=? AND target_id=? FOR UPDATE`, id, kind, code); e != nil {
			if e == sqlx.ErrNotFound {
				return common.ErrForbidden
			}
			return e
		}
		if s.Status == "paid" {
			return nil
		}
		if s.Status != "pending" && s.Status != "confirmed" {
			return common.ErrStatusInvalid
		}
		var paymentStatus string
		if e := tx.QueryRowCtx(ctx, &paymentStatus, `SELECT status FROM askxuan_payment.payment WHERE order_type=? AND order_no=? AND channel='demo_balance' ORDER BY id DESC LIMIT 1 FOR UPDATE`, s.Type, s.No); e != nil {
			return e
		}
		if paymentStatus != "success" {
			return common.ErrStatusInvalid
		}
		if s.Type == "consultation" {
			var complete int
			if e := tx.QueryRowCtx(ctx, &complete, `SELECT (status IN ('ended','expired','completed') OR (status='active' AND expires_at<=NOW())) FROM askxuan_booking.consultation_order WHERE order_no=?`, s.No); e != nil {
				return e
			}
			if complete != 1 {
				return common.NewBizError(40906, "咨询结束后才可模拟提现")
			}
		}
		no := fmt.Sprintf("DEMO-WD-%d", id)
		if _, e := tx.ExecCtx(ctx, `INSERT INTO demo_withdrawal(withdrawal_no,applicant_type,applicant_id,amount,bank_card,status,audit_time,process_time) VALUES(?,?,?,?,'','success',NOW(),NOW())`, no, kind, code, s.Amount); e != nil {
			return e
		}
		res, e := tx.ExecCtx(ctx, `INSERT INTO demo_finance_transaction(transaction_no,source_type,source_no,payment_no,event_type,total_amount,status) VALUES(?,?,?,'','demo_payout',?,'posted')`, no, kind, fmt.Sprint(id), s.Amount)
		if e != nil {
			return e
		}
		tid, e := res.LastInsertId()
		if e != nil {
			return e
		}
		ctx = DemoLedgerContext(ctx)
		account := LedgerAccountTemplePayable
		if kind == "master" {
			account = LedgerAccountMasterPayable
		}
		if e = insertLedgerEntry(ctx, tx, tid, account, code, "debit", s.Amount); e != nil {
			return e
		}
		if e = insertLedgerEntry(ctx, tx, tid, LedgerAccountPlatformCash, "", "credit", s.Amount); e != nil {
			return e
		}
		_, e = tx.ExecCtx(ctx, `UPDATE demo_settlement SET status='paid' WHERE id=?`, id)
		return e
	})
}

// ReadDemoPlatformWallet is restricted to platform administrators by the handler.
func ReadDemoPlatformWallet(ctx context.Context) (map[string]any, error) {
	var totals struct {
		Received int64 `db:"received" json:"receivedCents"`
		Refunded int64 `db:"refunded" json:"refundedCents"`
		Paid     int64 `db:"paid" json:"paidCents"`
	}
	if e := db.QueryRowCtx(ctx, &totals, `SELECT COALESCE(SUM(IF(event_type='payment_receipt',CAST(total_amount*100 AS SIGNED),0)),0) received,COALESCE(SUM(IF(event_type='refund',CAST(total_amount*100 AS SIGNED),0)),0) refunded,COALESCE(SUM(IF(event_type='demo_payout',CAST(total_amount*100 AS SIGNED),0)),0) paid FROM demo_finance_transaction`); e != nil {
		return nil, e
	}
	var commission int64
	if e := db.QueryRowCtx(ctx, &commission, `SELECT COALESCE(SUM(CAST(amount*100 AS SIGNED)*IF(direction='credit',1,-1)),0) FROM demo_finance_ledger_entry WHERE account_code='platform_commission'`); e != nil {
		return nil, e
	}
	rows := []WalletSettlement{}
	e := db.QueryRowsCtx(ctx, &rows, `SELECT id,settlement_no number,source_type,source_no,CAST(total_amount*100 AS SIGNED) gross,CAST(commission_amount*100 AS SIGNED) commission,CAST(settle_amount*100 AS SIGNED) net,status,DATE_FORMAT(create_time,'%Y-%m-%d %H:%i:%s') created_at FROM demo_settlement ORDER BY id DESC LIMIT 30`)
	return map[string]any{"mode": "demo", "totals": totals, "commissionCents": commission, "settlements": rows}, e
}

func IsDemoPayment(ctx context.Context, kind, no string) (bool, error) {
	scoped, err := paymentLedgerContext(ctx, kind, no)
	return isDemoLedger(scoped), err
}

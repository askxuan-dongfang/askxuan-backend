package model

import (
	"context"
	"fmt"
)

type WalletSettlement struct {
	ID              int64  `db:"id" json:"id"`
	Number          string `db:"number" json:"number"`
	SourceType      string `db:"source_type" json:"sourceType"`
	SourceNo        string `db:"source_no" json:"sourceNo"`
	GrossCents      int64  `db:"gross" json:"grossCents"`
	CommissionCents int64  `db:"commission" json:"commissionCents"`
	NetCents        int64  `db:"net" json:"netCents"`
	Status          string `db:"status" json:"status"`
	CreatedAt       string `db:"created_at" json:"createdAt"`
}
type WalletWithdrawal struct {
	ID          int64  `db:"id" json:"id"`
	Number      string `db:"number" json:"number"`
	AmountCents int64  `db:"amount" json:"amountCents"`
	Status      string `db:"status" json:"status"`
	CreatedAt   string `db:"created_at" json:"createdAt"`
}
type WalletSummary struct {
	PendingCents      int64 `db:"pending" json:"pendingCents"`
	ConfirmedCents    int64 `db:"confirmed" json:"confirmedCents"`
	RecordedPaidCents int64 `db:"paid" json:"recordedPaidCents"`
}
type ProviderWallet struct {
	Summary         WalletSummary      `json:"summary"`
	Settlements     []WalletSettlement `json:"settlements"`
	Withdrawals     []WalletWithdrawal `json:"withdrawals"`
	Total           int64              `json:"total"`
	WithdrawalTotal int64              `json:"withdrawalTotal"`
	Page            int                `json:"page"`
	PageSize        int                `json:"pageSize"`
	WithdrawEnabled bool               `json:"withdrawEnabled"`
	RecordMode      string             `json:"recordMode"`
}

// target is resolved from the authenticated master ID or temple code, never a query parameter.
func ReadProviderWallet(ctx context.Context, kind, target string, page int) (ProviderWallet, error) {
	out := ProviderWallet{Settlements: []WalletSettlement{}, Withdrawals: []WalletWithdrawal{}, Page: page, PageSize: 20, RecordMode: "accounting_only"}
	if isDemoLedger(ctx) {
		out.RecordMode = "demo"
		out.WithdrawEnabled = true
	}
	if (kind != "master" && kind != "temple") || target == "" || target == "0" || page < 1 || page > 100000 {
		return out, fmt.Errorf("invalid wallet owner")
	}
	code := target
	if kind == "master" {
		if err := db.QueryRowCtx(ctx, &code, `SELECT code FROM askxuan_master.master WHERE id=?`, target); err != nil {
			return out, err
		}
	}
	err := db.QueryRowCtx(ctx, &out.Summary, financeQuery(ctx, `SELECT COALESCE(SUM(IF(status='pending',CAST(settle_amount*100 AS SIGNED),0)),0) pending,COALESCE(SUM(IF(status='confirmed',CAST(settle_amount*100 AS SIGNED),0)),0) confirmed,COALESCE(SUM(IF(status='paid',CAST(settle_amount*100 AS SIGNED),0)),0) paid FROM settlement WHERE settle_type=? AND target_id=?`), kind, code)
	if err != nil {
		return out, err
	}
	if err = db.QueryRowCtx(ctx, &out.Total, financeQuery(ctx, `SELECT COUNT(*) FROM settlement WHERE settle_type=? AND target_id=?`), kind, code); err != nil {
		return out, err
	}
	err = db.QueryRowsCtx(ctx, &out.Settlements, financeQuery(ctx, `SELECT id,settlement_no number,source_type,source_no,CAST(total_amount*100 AS SIGNED) gross,CAST(commission_amount*100 AS SIGNED) commission,CAST(settle_amount*100 AS SIGNED) net,status,DATE_FORMAT(create_time,'%Y-%m-%d %H:%i:%s') created_at FROM settlement WHERE settle_type=? AND target_id=? ORDER BY id DESC LIMIT 20 OFFSET ?`), kind, code, (page-1)*20)
	if err != nil {
		return out, err
	}
	if err = db.QueryRowCtx(ctx, &out.WithdrawalTotal, financeQuery(ctx, `SELECT COUNT(*) FROM withdrawal WHERE applicant_type=? AND applicant_id IN (?,?)`), kind, target, code); err != nil {
		return out, err
	}
	// No bank account is returned. Historical success is only a simulation, not a bank receipt.
	err = db.QueryRowsCtx(ctx, &out.Withdrawals, financeQuery(ctx, `SELECT id,withdrawal_no number,CAST(amount*100 AS SIGNED) amount,status,DATE_FORMAT(create_time,'%Y-%m-%d %H:%i:%s') created_at FROM withdrawal WHERE applicant_type=? AND applicant_id IN (?,?) ORDER BY id DESC LIMIT 20 OFFSET ?`), kind, target, code, (page-1)*20)
	return out, err
}

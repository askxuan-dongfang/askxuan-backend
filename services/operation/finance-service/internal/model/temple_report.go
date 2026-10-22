package model

import (
	"context"
	"github.com/askxuan/common"
	"time"
)

type TempleSettlementGroup struct {
	SourceType      string `db:"source_type" json:"sourceType"`
	Status          string `db:"status" json:"status"`
	Count           int64  `db:"count" json:"count"`
	GrossCents      int64  `db:"gross_cents" json:"grossCents"`
	CommissionCents int64  `db:"commission_cents" json:"commissionCents"`
	NetCents        int64  `db:"net_cents" json:"netCents"`
}
type TempleSettlementReport struct {
	Groups     []TempleSettlementGroup `json:"groups"`
	RecordMode string                  `json:"recordMode"`
}

// Uses the same ledger and ownership as the temple wallet, never order gross as income.
func ReadTempleSettlementReport(ctx context.Context, code, start, end string) (TempleSettlementReport, error) {
	out := TempleSettlementReport{Groups: []TempleSettlementGroup{}, RecordMode: "accounting_only"}
	if isDemoLedger(ctx) {
		out.RecordMode = "demo"
	}
	if code == "" {
		return out, common.ErrForbidden
	}
	for _, d := range []string{start, end} {
		if d != "" {
			if _, err := time.Parse("2006-01-02", d); err != nil {
				return out, common.ErrParam
			}
		}
	}
	if start != "" && end != "" && start > end {
		return out, common.ErrParam
	}
	where := "settle_type='temple' AND target_id=?"
	args := []interface{}{code}
	if start != "" {
		where += " AND create_time>=?"
		args = append(args, start+" 00:00:00")
	}
	if end != "" {
		where += " AND create_time<DATE_ADD(?,INTERVAL 1 DAY)"
		args = append(args, end+" 00:00:00")
	}
	err := db.QueryRowsCtx(ctx, &out.Groups, financeQuery(ctx, `SELECT source_type,status,COUNT(*) count,
 CAST(ROUND(SUM(total_amount)*100) AS SIGNED) gross_cents,
 CAST(ROUND(SUM(commission_amount)*100) AS SIGNED) commission_cents,
 CAST(ROUND(SUM(settle_amount)*100) AS SIGNED) net_cents
 FROM settlement WHERE `+where+` GROUP BY source_type,status ORDER BY source_type,status`), args...)
	return out, err
}

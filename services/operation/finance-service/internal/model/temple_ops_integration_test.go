package model

import (
	"context"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"os"
	"strings"
	"testing"
)

func TestTempleOpsSettlementReportIntegration(t *testing.T) {
	dsn := os.Getenv("TEMPLE_OPS_FINANCE_DSN")
	if dsn == "" {
		t.Skip("requires task-only MySQL")
	}
	if !strings.Contains(dsn, "@tcp(127.0.0.1:63336)/askxuan_finance?") {
		t.Fatal("refuse non-isolated database")
	}
	sqlx.DisableLog()
	conn := sqlx.NewMysql(dsn)
	ctx := context.Background()
	old := db
	Configure(conn)
	defer Configure(old)
	exec := func(q string, args ...any) {
		t.Helper()
		if _, e := conn.ExecCtx(ctx, q, args...); e != nil {
			t.Fatal(e)
		}
	}
	exec(`DELETE FROM settlement WHERE settlement_no LIKE 'TEMPLEOPS_%'`)
	t.Cleanup(func() { exec(`DELETE FROM settlement WHERE settlement_no LIKE 'TEMPLEOPS_%'`) })
	exec(`INSERT INTO settlement(settlement_no,settle_type,target_id,total_amount,commission_amount,settle_amount,status,source_type,source_no,create_time) VALUES('TEMPLEOPS_1','temple','T_OPS',100.50,10.05,90.45,'confirmed','booking','TEMPLEOPS_B1','2099-10-22 23:59:59'),('TEMPLEOPS_2','master','T_OPS',200,20,180,'confirmed','booking','TEMPLEOPS_B1','2099-10-22'),('TEMPLEOPS_3','temple','OTHER',999,0,999,'paid','diy','TEMPLEOPS_D1','2099-10-22'),('TEMPLEOPS_4','temple','T_OPS',10,1,9,'pending','diy','TEMPLEOPS_D2','2099-10-23')`)
	out, e := ReadTempleSettlementReport(ctx, "T_OPS", "2099-10-22", "2099-10-22")
	if e != nil {
		t.Fatal(e)
	}
	if len(out.Groups) != 1 {
		t.Fatalf("scope/date: %+v", out)
	}
	g := out.Groups[0]
	if g.GrossCents != 10050 || g.CommissionCents != 1005 || g.NetCents != 9045 || g.SourceType != "booking" || g.Count != 1 {
		t.Fatalf("fee split %+v", g)
	}
	wallet, e := ReadProviderWallet(ctx, "temple", "T_OPS", 1)
	if e != nil {
		t.Fatal(e)
	}
	if wallet.Summary.ConfirmedCents != 9045 || wallet.Summary.PendingCents != 900 {
		t.Fatal("report/wallet mismatch", wallet.Summary)
	}
	if _, e = ReadTempleSettlementReport(ctx, "T_OPS", "2099-10-23", "2099-10-22"); e == nil {
		t.Fatal("inverted dates accepted")
	}
}

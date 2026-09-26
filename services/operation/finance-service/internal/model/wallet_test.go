package model

import (
	"context"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"os"
	"strings"
	"testing"
)

func TestMySQLProviderWalletAllocation(t *testing.T) {
	dsn := os.Getenv("WALLET_TEST_DSN")
	if dsn == "" {
		t.Skip("isolated WALLET_TEST_DSN not set")
	}
	if !strings.Contains(dsn, "/wallet_test") {
		t.Fatal("isolated wallet_test required")
	}
	old := db
	Configure(sqlx.NewMysql(dsn))
	defer func() { db = old }()
	ctx := context.Background()
	for _, q := range []string{
		`CREATE DATABASE IF NOT EXISTS askxuan_master`, `CREATE TABLE IF NOT EXISTS askxuan_master.master(id BIGINT PRIMARY KEY,code VARCHAR(32))`, `REPLACE INTO askxuan_master.master VALUES(901,'M901'),(902,'M902'),(999,'M999')`,
		`DROP TABLE IF EXISTS settlement`, `DROP TABLE IF EXISTS withdrawal`,
		`CREATE TABLE settlement(id BIGINT PRIMARY KEY,settlement_no VARCHAR(32),settle_type VARCHAR(32),target_id VARCHAR(32),total_amount DECIMAL(10,2),commission_amount DECIMAL(10,2),settle_amount DECIMAL(10,2),source_type VARCHAR(32),source_no VARCHAR(32),status VARCHAR(32),create_time DATETIME)`,
		`CREATE TABLE withdrawal(id BIGINT PRIMARY KEY,withdrawal_no VARCHAR(32),applicant_type VARCHAR(32),applicant_id VARCHAR(32),amount DECIMAL(10,2),status VARCHAR(32),create_time DATETIME)`,
		`INSERT INTO settlement VALUES(1,'S1','master','M901',40,6,34,'booking','B1','pending',NOW()),(2,'S2','temple','T901',60,9,51,'booking','B1','confirmed',NOW()),(3,'S3','master','M902',200,30,170,'booking','B2','paid',NOW())`,
		`INSERT INTO withdrawal VALUES(1,'W1','master','901',3,'success',NOW()),(2,'W2','temple','T901',5,'success',NOW())`,
	} {
		if _, e := db.ExecCtx(ctx, q); e != nil {
			t.Fatal(e)
		}
	}
	m, e := ReadProviderWallet(ctx, "master", "901", 1)
	if e != nil {
		t.Fatal(e)
	}
	if m.Total != 1 || m.Summary.PendingCents != 3400 || m.Summary.ConfirmedCents != 0 || len(m.Withdrawals) != 1 || m.Withdrawals[0].AmountCents != 300 || m.WithdrawEnabled {
		t.Fatalf("master scope: %+v", m)
	}
	temple, e := ReadProviderWallet(ctx, "temple", "T901", 1)
	if e != nil || temple.Summary.ConfirmedCents != 5100 || temple.Total != 1 || temple.Withdrawals[0].AmountCents != 500 {
		t.Fatalf("temple scope %+v %v", temple, e)
	}
	if m.Settlements[0].NetCents+temple.Settlements[0].NetCents != 8500 {
		t.Fatal("double counted order allocation")
	}
	empty, e := ReadProviderWallet(ctx, "master", "999", 1)
	if e != nil || empty.Total != 0 || empty.WithdrawalTotal != 0 {
		t.Fatal("cross account leakage")
	}
}

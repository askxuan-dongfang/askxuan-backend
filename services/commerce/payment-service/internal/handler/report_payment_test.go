package handler

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"testing"
)

func TestReportPurchaseRejectsInvalidStateAndPrice(t *testing.T) {
	for _, tc := range []struct {
		name, status          string
		price, paid, expected int64
		ok                    bool
	}{{"not generated", "generating", 10, 0, 10, false}, {"failed", "failed", 10, 0, 10, false}, {"price tampering", "ready", 10, 0, 1, false}, {"idempotent paid retry", "ready", 10, 1, 10, true}} {
		t.Run(tc.name, func(t *testing.T) {
			db, m, _ := sqlmock.New()
			defer db.Close()
			m.ExpectBegin()
			m.ExpectQuery("SELECT report_no.*FOR UPDATE").WithArgs(int64(1), "100").WillReturnRows(sqlmock.NewRows([]string{"report_no", "points_price", "points_paid", "status"}).AddRow("AR1", tc.price, tc.paid, tc.status))
			if tc.ok {
				m.ExpectCommit()
			} else {
				m.ExpectRollback()
			}
			e := redeemReport(context.Background(), sqlx.NewSqlConnFromDB(db), "100", 1, tc.expected)
			if (e == nil) != tc.ok {
				t.Fatalf("error=%v", e)
			}
			if e := m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestReportPurchaseAtomicLedgerAndUnlock(t *testing.T) {
	for _, insufficient := range []bool{false, true} {
		db, m, _ := sqlmock.New()
		m.ExpectBegin()
		m.ExpectQuery("SELECT report_no.*FOR UPDATE").WithArgs(int64(1), "100").WillReturnRows(sqlmock.NewRows([]string{"report_no", "points_price", "points_paid", "status"}).AddRow("AR1", 10, 0, "ready"))
		m.ExpectQuery("SELECT COUNT.*FROM payment").WithArgs("AR1").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		m.ExpectExec("INSERT INTO points_account").WithArgs("100").WillReturnResult(sqlmock.NewResult(0, 1))
		balance := int64(20)
		if insufficient {
			balance = 2
		}
		m.ExpectQuery("SELECT balance.*FOR UPDATE").WithArgs("100").WillReturnRows(sqlmock.NewRows([]string{"balance"}).AddRow(balance))
		m.ExpectQuery("SELECT COUNT.*points_ledger").WithArgs("ai-report:AR1").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		if insufficient {
			m.ExpectRollback()
		} else {
			m.ExpectExec("UPDATE points_account").WithArgs(int64(-10), "100").WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectExec("INSERT INTO points_ledger").WithArgs("100", "ai-report:AR1", "redeem", int64(-10), int64(10), "AR1").WillReturnResult(sqlmock.NewResult(1, 1))
			m.ExpectExec("UPDATE askxuan_ai.ai_report SET points_paid=1").WithArgs(int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectCommit()
		}
		e := redeemReport(context.Background(), sqlx.NewSqlConnFromDB(db), "100", 1, 10)
		if (e != nil) != insufficient {
			t.Fatalf("insufficient %v error %v", insufficient, e)
		}
		if e := m.ExpectationsWereMet(); e != nil {
			t.Fatal(e)
		}
		db.Close()
	}
}

package model

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"testing"
)

func TestAuxiliaryUsageIdempotent(t *testing.T) {
	for _, affected := range []int64{0, 1} {
		db, m, _ := sqlmock.New()
		conn := sqlx.NewSqlConnFromDB(db)
		m.ExpectBegin()
		m.ExpectExec("INSERT IGNORE INTO ai_aux_usage_log").WillReturnResult(sqlmock.NewResult(0, affected))
		if affected == 1 {
			m.ExpectExec("UPDATE ai_usage_counter").WithArgs(30, int64(7), "17", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(0, 1))
		}
		m.ExpectCommit()
		e := RecordAuxiliaryUsage(context.Background(), conn, "request-1", UsageRecord{UserID: "17", PromptTokens: 10, CompletionTokens: 20, CostMicros: 7})
		if e != nil {
			t.Fatal(e)
		}
		if e = m.ExpectationsWereMet(); e != nil {
			t.Fatal(e)
		}
		db.Close()
	}
}

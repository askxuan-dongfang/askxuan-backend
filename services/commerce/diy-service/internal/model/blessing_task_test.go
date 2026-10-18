package model

import (
	"context"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"testing"
)

// ===== 加持任务状态机测试（DIY侧） =====

func TestCanTransitBlessingTask(t *testing.T) {
	tests := []struct {
		name   string
		from   string
		to     string
		expect bool
	}{
		// 合法流转
		{"dispatched→assigned", BlessingTaskStatusDispatched, BlessingTaskStatusAssigned, true},
		{"assigned→accepted", BlessingTaskStatusAssigned, BlessingTaskStatusAccepted, true},
		{"assigned→rejected", BlessingTaskStatusAssigned, BlessingTaskStatusRejected, true},
		{"rejected→assigned(重新分配)", BlessingTaskStatusRejected, BlessingTaskStatusAssigned, true},
		{"accepted→in_progress", BlessingTaskStatusAccepted, BlessingTaskStatusInProgress, true},
		{"in_progress→completed", BlessingTaskStatusInProgress, BlessingTaskStatusCompleted, true},

		// 非法流转
		{"dispatched→accepted(跳过分配)", BlessingTaskStatusDispatched, BlessingTaskStatusAccepted, false},
		{"dispatched→completed(跳过流程)", BlessingTaskStatusDispatched, BlessingTaskStatusCompleted, false},
		{"assigned→in_progress(跳过接受)", BlessingTaskStatusAssigned, BlessingTaskStatusInProgress, false},
		{"accepted→completed(跳过加持中)", BlessingTaskStatusAccepted, BlessingTaskStatusCompleted, false},
		{"completed→in_progress(已完成不能回退)", BlessingTaskStatusCompleted, BlessingTaskStatusInProgress, false},
		{"rejected→accepted(拒绝后必须重新分配)", BlessingTaskStatusRejected, BlessingTaskStatusAccepted, false},
		{"相同状态", BlessingTaskStatusDispatched, BlessingTaskStatusDispatched, false},
		{"未知源状态", "unknown", BlessingTaskStatusAssigned, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanTransitBlessingTask(tt.from, tt.to)
			if got != tt.expect {
				t.Errorf("CanTransitBlessingTask(%q→%q) 期望 %v, 实际 %v", tt.from, tt.to, tt.expect, got)
			}
		})
	}
}

func taskRows(state string) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "task_no", "diy_order_no", "temple_code", "master_code", "status", "certificate_urls", "assign_time", "complete_time", "create_time", "update_time"}).AddRow(2, "BT-test", "DIY-test", "T001", "M001", state, "[]", nil, nil, "2026-10-19", "2026-10-19")
}

func TestStartRollsBackWhenCustomerOrderCannotAdvance(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := NewBlessingTaskModel(sqlx.NewSqlConnFromDB(db))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FOR UPDATE").WithArgs(int64(2)).WillReturnRows(taskRows(BlessingTaskStatusAccepted))
	mock.ExpectExec("UPDATE askxuan_diy.diy_order").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectRollback()
	if _, err = m.UpdateStatus(context.Background(), 2, BlessingTaskStatusInProgress); err == nil {
		t.Fatal("invalid order accepted")
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCompletionRollsBackBothUpdatesOnCertificateWriteFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := NewBlessingTaskModel(sqlx.NewSqlConnFromDB(db))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FOR UPDATE").WillReturnRows(taskRows(BlessingTaskStatusInProgress))
	mock.ExpectExec("UPDATE askxuan_diy.diy_order").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE askxuan_diy.blessing_task").WillReturnError(errors.New("storage unavailable"))
	mock.ExpectRollback()
	if err = m.UpdateComplete(context.Background(), 2, `["https://example.test/certificate.png"]`); err == nil {
		t.Fatal("write failure ignored")
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAssignmentRejectsMasterOutsideTemple(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	m := NewBlessingTaskModel(sqlx.NewSqlConnFromDB(db))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FOR UPDATE").WillReturnRows(taskRows(BlessingTaskStatusDispatched))
	mock.ExpectQuery("SELECT COUNT.*askxuan_master.master").WithArgs("M002", "T001").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectRollback()
	if _, err = m.Assign(context.Background(), 2, "M002"); err == nil {
		t.Fatal("foreign master assigned")
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCertificateValidation(t *testing.T) {
	for _, raw := range []string{`[]`, `null`, `bad`, `[""]`, `["javascript:alert(1)"]`, `["https://user:pass@example.test/a"]`} {
		if validateCertificateURLs(raw) == nil {
			t.Errorf("accepted invalid certificate %s", raw)
		}
	}
	if err := validateCertificateURLs(`["https://example.test/a.png","http://127.0.0.1:39000/a.png"]`); err != nil {
		t.Fatal(err)
	}
}

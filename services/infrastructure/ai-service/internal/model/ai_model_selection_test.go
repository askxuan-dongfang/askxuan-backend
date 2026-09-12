package model

import (
	"context"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"strings"
	"testing"
)

func TestSelectedModelIsStoredWithPendingMessage(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewConversationModel(sqlx.NewSqlConnFromDB(db))
	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO ai_session").WithArgs(sqlmock.AnyArg(), "42", "general", "explicit", "1", "hello", SessionStatusActive).WillReturnResult(sqlmock.NewResult(7, 1))
	mock.ExpectExec("INSERT INTO ai_message.*input_json").WithArgs(int64(7), RoleUser, "hello", "{}", "[]", MessageStatusCompleted).WillReturnResult(sqlmock.NewResult(8, 1))
	mock.ExpectExec("INSERT INTO ai_message.*status,model").WithArgs(int64(7), RoleAssistant, "", MessageStatusPending, "deepseek-v4-pro").WillReturnResult(sqlmock.NewResult(9, 1))
	mock.ExpectCommit()
	_, pending, err := store.CreateSession(context.Background(), "42", "general", "explicit", "1", "hello", "{}", "[]", "deepseek-v4-pro")
	if err != nil || pending != 9 {
		t.Fatal(pending, err)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestTurnAndChosenModelRollbackTogether(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := NewConversationModel(sqlx.NewSqlConnFromDB(db))
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT .* FROM ai_session.*FOR UPDATE").WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows(strings.Split(sessionRows, ",")).AddRow(7, "AI7", "42", "general", "explicit", "1", "hello", SessionStatusActive, "2026-09-12", "2026-09-12"))
	mock.ExpectExec("INSERT INTO ai_message.*input_json").WithArgs(int64(7), RoleUser, "followup", "{}", "[]", MessageStatusCompleted).WillReturnResult(sqlmock.NewResult(10, 1))
	mock.ExpectExec("INSERT INTO ai_message.*status,model").WithArgs(int64(7), RoleAssistant, "", MessageStatusPending, "deepseek-flash").WillReturnError(errors.New("isolated insert failure"))
	mock.ExpectRollback()
	if _, err = store.CreateTurn(context.Background(), 7, "42", "followup", "{}", "[]", "deepseek-flash"); err == nil {
		t.Fatal("failed pending insert accepted")
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

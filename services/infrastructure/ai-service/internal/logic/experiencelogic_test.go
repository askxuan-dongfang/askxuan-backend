package logic

import (
	"context"
	"encoding/json"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/askxuan/ai-service/internal/experience"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"testing"
)

func TestExperienceRetryPreservesRetiredRuleSnapshot(t *testing.T) {
	db, m, e := sqlmock.New()
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	req := ExperienceSave{RequestKey: "old-version-key", Input: experience.Input{Skill: "naming", Naming: &experience.NamingInput{Purpose: "pen", Style: "retired-style"}}, Favorites: []string{"旧候选"}, Note: "保留当时的想法"}
	old := ExperienceData{Input: req.Input, Result: experience.Result{Skill: "naming", Version: "old"}, Favorites: req.Favorites, Note: req.Note}
	payload, _ := json.Marshal(old)
	m.ExpectQuery("SELECT id.*ai_experience_note WHERE user_id").WithArgs("7", req.RequestKey).WillReturnRows(sqlmock.NewRows([]string{"id", "payload", "created_at"}).AddRow(9, string(payload), "2026-09-21"))
	got, e := ExperienceSaveNote(context.Background(), &svc.ServiceContext{DB: sqlx.NewSqlConnFromDB(db)}, "7", req)
	if e != nil || got.ID != 9 || got.Data.Result.Version != "old" {
		t.Fatalf("retry recomputed history: %+v %v", got, e)
	}
	if e := m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}

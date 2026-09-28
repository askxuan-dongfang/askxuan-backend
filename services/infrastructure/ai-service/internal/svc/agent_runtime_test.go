package svc

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/askxuan/ai-service/internal/agentops"
	"github.com/askxuan/ai-service/internal/config"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestAskRuntimePinsPublishedVersionAndFailsClosed(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ops := agentops.New(&agentops.SQLRepository{DB: sqlx.NewSqlConnFromDB(db)}, nil, nil, nil, true)
	source := &ServiceContext{AgentOps: ops, AIConfig: config.AIConf{MaxOutputTokens: 2048}}
	expect := func(id int64, instruction string) {
		f := agentops.Frozen{Config: agentops.Config{Name: "问事", Model: "fixture-model", Instruction: instruction, MaxOutputTokens: 512}, Skills: []model.AISkill{{Code: "general", Status: "enabled", Version: "1.0.0", PromptTemplate: "skill prompt"}}}
		raw, _ := json.Marshal(f)
		mock.ExpectQuery("SELECT revision").WillReturnRows(sqlmock.NewRows([]string{"revision", "draft_json", "active_version"}).AddRow(9, "", id))
		mock.ExpectQuery("SELECT id,definition_json").WithArgs(id).WillReturnRows(sqlmock.NewRows([]string{"id", "definition_json", "actor", "note", "create_time"}).AddRow(id, string(raw), "1", "test", "2026-09-28"))
	}
	expect(1, "first version")
	first, err := source.AskRuntime(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	expect(2, "next version")
	next, err := source.AskRuntime(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	a, _ := first.SkillModel.FindByCode(context.Background(), "general")
	b, _ := next.SkillModel.FindByCode(context.Background(), "general")
	if !strings.Contains(a.PromptTemplate, "first version") || !strings.Contains(a.Version, "@a1") || !strings.Contains(b.PromptTemplate, "next version") || !strings.Contains(b.Version, "@a2") {
		t.Fatal("in-flight configuration changed during publish")
	}
	if first.AIConfig.MaxOutputTokens != 512 || source.AIConfig.MaxOutputTokens != 2048 || first.AgentDefaultModel != "fixture-model" {
		t.Fatal("request limits not isolated")
	}
	mock.ExpectQuery("SELECT revision").WillReturnError(errors.New("storage outage"))
	if v, e := source.AskRuntime(context.Background()); e == nil || v != nil {
		t.Fatal("silently fell back after active-store failure")
	}
	ops.LiveEnabled = false
	if v, e := source.AskRuntime(context.Background()); e != nil || v != source {
		t.Fatal("emergency switch failed")
	}
	if e := mock.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}

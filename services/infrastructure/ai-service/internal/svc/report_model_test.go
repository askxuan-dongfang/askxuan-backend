package svc

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/askxuan/ai-service/internal/agentops"
	"github.com/askxuan/ai-service/internal/config"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/settings"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestReportModelSurvivesSettingsAndPublishedAgentSnapshot(t *testing.T) {
	t.Setenv("AI_REPORT_MODEL", "report-model")
	t.Setenv("AI_PROVIDER", "mock")
	t.Setenv("AI_MODEL", "mock")
	t.Setenv("AI_THINKING_ENABLED", "false")
	t.Setenv("AI_REASONING_EFFORT", "low")
	c := (config.AIConf{Provider: "mock", Model: "mock"}).Runtime()
	manager, err := settings.New(c, "", "")
	if err != nil {
		t.Fatal(err)
	}
	snapshot, _ := manager.VersionedSnapshot()
	if snapshot.Config.ReportModel != "report-model" {
		t.Fatal("provider settings snapshot discarded deployment report model")
	}
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	conn := sqlx.NewSqlConnFromDB(db)
	frozen, _ := json.Marshal(agentops.Frozen{Config: agentops.Config{Name: "fixture", Model: "published-chat", MaxOutputTokens: 8192}, Skills: []model.AISkill{{Code: "general", Status: model.SkillStatusEnabled}}})
	mock.ExpectQuery("SELECT version_id,stable_version,percentage,revision FROM ai_agent_rollout").WillReturnRows(sqlmock.NewRows([]string{"version_id", "stable_version", "percentage", "revision"}).AddRow(3, 3, 100, 1))
	mock.ExpectQuery("SELECT id,definition_json,actor,note,create_time FROM ai_agent_version").WithArgs(int64(3)).WillReturnRows(sqlmock.NewRows([]string{"id", "definition_json", "actor", "note", "create_time"}).AddRow(3, string(frozen), "fixture", "fixture", "2026-10-28"))
	ops := agentops.New(&agentops.SQLRepository{DB: conn}, nil, manager.VersionedSnapshot, nil, true)
	s := (&ServiceContext{AIConfig: snapshot.Config, Provider: snapshot.Provider, Models: snapshot.Models, Settings: manager, AgentOps: ops}).Runtime()
	live, err := s.AskRuntimeFor(context.Background(), "synthetic-subject")
	if err != nil || live.AIConfig.ReportModel != "report-model" || live.AgentDefaultModel != "published-chat" || live.AgentVersion != 3 {
		t.Fatalf("report model lost to published agent selection: %v", err)
	}
	if live.AIConfig.ThinkingEnabled || live.AIConfig.ReasoningEffort != "low" || live.Provider.Model() != "mock" || snapshot.Config.ReportModel != "report-model" {
		t.Fatal("request-local report configuration changed chat/provider defaults")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

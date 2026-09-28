package handler

// Opt-in local UI fixture. Never linked into the production service binary.
import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/askxuan/ai-service/internal/agentops"
	"github.com/askxuan/ai-service/internal/config"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/askxuan/ai-service/internal/settings"
	"github.com/askxuan/ai-service/internal/svc"
	mysql "github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/rest"
)

type browserSkills struct{}

func (browserSkills) List(context.Context, string) ([]*model.AISkill, error) {
	return []*model.AISkill{
		{Code: "general", Name: "日常问事", Version: "1.0.0", Status: "enabled", Description: "用于本地联调的日常问事技能", PromptTemplate: "请提供审慎的生活参考。", InputSchema: `{"fields":[]}`, ToolConfig: `{"enabled":false}`},
		{Code: "bazi", Name: "八字资料解读", Version: "1.0.0", Status: "enabled", Description: "使用合成出生资料验证计算与补问流程", PromptTemplate: "根据工具依据解释资料，避免确定预言。", InputSchema: `{"fields":[{"key":"birthDate","label":"出生日期","type":"date","required":true},{"key":"birthTime","label":"出生时间","type":"time","required":true},{"key":"gender","label":"性别","type":"select","required":true,"options":[{"value":"male","label":"男"},{"value":"female","label":"女"}]}]}`, ToolConfig: `{"enabled":true,"server":"fixture","tool":"bazi"}`},
	}, nil
}
func (s browserSkills) FindByCode(ctx context.Context, code string) (*model.AISkill, error) {
	all, _ := s.List(ctx, "")
	for _, v := range all {
		if v.Code == code {
			return v, nil
		}
	}
	return nil, sqlx.ErrNotFound
}

type browserMCP struct{}

func (browserMCP) Call(context.Context, string, string) (string, error) {
	return `{"source":"local-fixture","result":"合成计算依据，仅用于界面联调"}`, nil
}

func TestAgentOperationsBrowserFixture(t *testing.T) {
	if os.Getenv("AI_AGENT_OPS_BROWSER") != "true" {
		t.Skip("opt-in local browser fixture")
	}
	dsn := os.Getenv("AI_AGENT_OPS_BROWSER_DSN")
	cfg, e := mysql.ParseDSN(dsn)
	if e != nil || cfg.DBName != "askxuan_agentops_browser" || !strings.HasPrefix(cfg.Addr, "127.0.0.1:") {
		t.Fatal("requires isolated loopback browser DB")
	}
	db := sqlx.NewMysql(dsn)
	sqlx.DisableLog()
	ctx := context.Background()
	migration, e := os.ReadFile("../../../../../scripts/db/20260928_ai_agent_operations.sql")
	if e != nil {
		t.Fatal(e)
	}
	for _, stmt := range strings.Split(string(migration), ";") {
		if strings.TrimSpace(stmt) != "" {
			if _, e = db.ExecCtx(ctx, stmt); e != nil {
				t.Fatal(e)
			}
		}
	}
	for _, stmt := range []string{
		`CREATE TABLE IF NOT EXISTS ai_message(id BIGINT PRIMARY KEY,prompt_tokens INT,completion_tokens INT,cost_micros BIGINT)`,
		`CREATE TABLE IF NOT EXISTS ai_run(id BIGINT PRIMARY KEY,run_no VARCHAR(64),message_id BIGINT,skill_code VARCHAR(64),skill_version VARCHAR(64),model VARCHAR(64),status VARCHAR(16),stage VARCHAR(32),started_at DATETIME,completed_at DATETIME)`,
		`CREATE TABLE IF NOT EXISTS ai_tool_call(id BIGINT PRIMARY KEY,run_id BIGINT,tool_name VARCHAR(64),status VARCHAR(16),latency_ms INT,create_time DATETIME)`,
		`INSERT IGNORE INTO ai_message VALUES(1,120,80,320)`,
		`INSERT IGNORE INTO ai_run VALUES(1,'LOCAL-FIXTURE-001',1,'bazi','1.0.0@a1','fixture','completed','completed',NOW()-INTERVAL 3 SECOND,NOW())`,
		`INSERT IGNORE INTO ai_tool_call VALUES(1,1,'bazi','completed',42,NOW())`,
	} {
		if _, e = db.ExecCtx(ctx, stmt); e != nil {
			t.Fatal(e)
		}
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			fmt.Fprint(w, `{"data":[{"id":"fixture"}]}`)
			return
		}
		var req struct {
			Tools    []any `json:"tools"`
			Messages []struct {
				Role string `json:"role"`
			} `json:"messages"`
		}
		if json.NewDecoder(r.Body).Decode(&req) != nil {
			http.Error(w, "invalid", 400)
			return
		}
		hasResult := false
		for _, msg := range req.Messages {
			if msg.Role == "tool" {
				hasResult = true
			}
		}
		w.Header().Set("Content-Type", "text/event-stream")
		delta := map[string]any{"role": "assistant", "content": "【本地协议联调】已完成资料检查。可以结合实际情况进行文化与生活参考；此响应不代表真实模型的回答质量。"}
		reason := "stop"
		if len(req.Tools) > 0 && !hasResult {
			delta = map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": "fixture-call", "type": "function", "function": map[string]any{"name": "calculate_bazi", "arguments": "{}"}}}}
			reason = "tool_calls"
		}
		raw, _ := json.Marshal(map[string]any{"id": "fixture-response", "object": "chat.completion.chunk", "model": "fixture", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": reason}}})
		fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", raw)
	}))
	defer upstream.Close()
	p := provider.NewOpenAICompatible(upstream.URL+"/v1", "fixture", "fixture", "")
	p.UseStandardParameters()
	p.SetHTTPClient(upstream.Client())
	snap := &settings.Snapshot{Provider: p, Models: provider.NewCatalog(p), Config: config.AIConf{Provider: "openai_compatible", MaxInputChars: 2000, MaxOutputTokens: 1024}}
	m := agentops.New(&agentops.SQLRepository{DB: db}, browserSkills{}, func() (*settings.Snapshot, int64) { return snap, 0 }, browserMCP{}, true)
	server, e := rest.NewServer(rest.RestConf{Host: "127.0.0.1", Port: 19085, Timeout: 10000})
	if e != nil {
		t.Fatal(e)
	}
	defer server.Stop()
	token := "fixture." + base64.RawURLEncoding.EncodeToString([]byte(`{"userId":1,"userType":"admin","roles":["platform_super"],"clientId":"platform-admin","exp":4102444800}`)) + ".fixture"
	server.Use(func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer "+token {
				http.Error(w, "local fixture token required", 401)
				return
			}
			r.Header.Set("X-User-Id", "1")
			r.Header.Set("X-User-Type", "admin")
			r.Header.Set("X-User-Roles", "platform_super")
			next(w, r)
		}
	})
	s := &svc.ServiceContext{DB: db, AgentOps: m, Provider: p, Models: snap.Models}
	registerAgentOperations(server, s)
	server.AddRoute(rest.Route{Method: "GET", Path: "/api/v1/ai/models", Handler: modelListHandler(s)})
	t.Log("Local fixture ready on 127.0.0.1:19085; model and MCP are synthetic")
	server.Start()
}

package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/askxuan/ai-service/internal/agent"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestReportEarlyFailureKeepsFixedPhaseAndActualToolAttempts(t *testing.T) {
	for _, tc := range []struct {
		name, server, inputs, phase, reportModel string
		wantHTTP                                 int
	}{
		{"native_error", "builtin", `{"scene":"home","floorPlan":"{\"PRIVATE\":\"FLOORPLAN\"}"}`, "calculating", "", 0},
		{"mcp_error", "taibu", `{"scene":"home"}`, "calculating", "", 1},
		{"selection_after_native", "builtin", `{"scene":"home"}`, "model_selection", "PRIVATE_MODEL", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			mcp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprint(w, "PRIVATE MCP ERROR Authorization Bearer PRIVATE_KEY")
			}))
			t.Cleanup(mcp.Close)
			p := provider.NewOpenAICompatible("http://private.invalid", "PRIVATE_KEY", "fixture", "")
			p.SetHTTPClient(&http.Client{Transport: defaultContextTransport(func(r *http.Request) (*http.Response, error) {
				t.Error("pre-writer failure called a model or its catalog")
				return nil, fmt.Errorf("PRIVATE PROVIDER ERROR")
			})})
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })
			for _, stage := range []string{"checking", "calculating"} {
				mock.ExpectExec("UPDATE ai_report SET generation_stage").WithArgs(stage, int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
			}
			skill := &model.AISkill{Code: "fengshui", Status: model.SkillStatusEnabled, InputSchema: reportToolSchema("fengshui"), ToolConfig: `{"enabled":true,"server":"` + tc.server + `","tool":"fengshui"}`}
			s := &svc.ServiceContext{DB: sqlx.NewSqlConnFromDB(db), SkillModel: reportSkills{skill}, Provider: p, MCP: agent.NewMCPClient(true, mcp.URL, 10), Guard: agent.NewGuard(20000, nil)}
			s.AIConfig.ReportModel = tc.reportModel
			body, doc, usage, err := executeReport(context.Background(), s, &Report{ID: 1, SkillCode: "fengshui", Question: "PRIVATE QUESTION", InputsJSON: tc.inputs})
			if err == nil || body != (reportBody{}) || calls != tc.wantHTTP || doc.ModelCalls != 0 || doc.ToolCalls != 1 || usage.PromptTokens != 0 || usage.CompletionTokens != 0 || usage.Model != "" {
				t.Fatalf("early failure lost its actual attempt count or ran generation: err=%v bodyEmpty=%t calls=%d models=%d tools=%d", err, body == (reportBody{}), calls, doc.ModelCalls, doc.ToolCalls)
			}
			audit := reportFailureAudit(doc)
			var event map[string]any
			if len(audit.Evidence) != 1 || audit.Evidence[0].Tool != "report_calibration" || json.Unmarshal([]byte(audit.Evidence[0].Text), &event) != nil {
				t.Fatal("early failure has no single safe audit")
			}
			if len(event) != 6 || event["version"] != float64(2) || event["status"] != "failed" || event["phase"] != tc.phase || event["reason"] != "execution_failed" || event["modelCalls"] != float64(0) || event["agentToolCalls"] != float64(1) {
				t.Fatalf("unsafe or incorrect early failure classification: %+v", event)
			}
			raw, _ := json.Marshal(audit)
			for _, forbidden := range []string{"PRIVATE", "Authorization", "private.invalid", "floorPlan", "ToolConfig", "rows", "charts"} {
				if strings.Contains(string(raw), forbidden) {
					t.Fatalf("early failure audit leaked %q", forbidden)
				}
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

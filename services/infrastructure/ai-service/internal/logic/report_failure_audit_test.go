package logic

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/askxuan/ai-service/internal/reportdoc"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestReportFailureAuditRetainsOnlyCountsAndPrivateCalibration(t *testing.T) {
	privateReview := reportdoc.Evidence{Tool: "report_calibration", Text: `{"status":"needs_repair","reviews":2,"issues":[{"quote":"PRIVATE REVIEW QUOTE","basis":"private comparison"}]}`}
	doc := reportdoc.Document{
		Version: 1, Runtime: "harness", ModelCalls: 7, ToolCalls: 3,
		Blocks: []reportdoc.Block{{Kind: "table", Title: "PRIVATE CHART", Rows: [][]string{{"PRIVATE DRAFT"}}}},
		Evidence: []reportdoc.Evidence{
			{Tool: "bazi", Text: "PRIVATE RAW CALCULATION"},
			{Tool: "participant", Text: "PRIVATE PARTICIPANT PROFILE"},
			{Tool: "recall_memory", Text: "PRIVATE MEMORY"},
			{Tool: "search_knowledge", Text: "PRIVATE RETRIEVAL INPUT"},
			privateReview,
			{Tool: "writer", Text: "PRIVATE FULL BODY"},
		},
	}
	audit := reportFailureAudit(doc)
	want := reportdoc.Document{Version: 1, Runtime: "harness", ModelCalls: 7, ToolCalls: 3, Blocks: []reportdoc.Block{}, Evidence: []reportdoc.Evidence{privateReview}}
	if !reflect.DeepEqual(audit, want) {
		t.Fatalf("failure audit retained non-audit data: %+v", audit)
	}
	raw, err := json.Marshal(audit)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"PRIVATE CHART", "PRIVATE DRAFT", "PRIVATE RAW CALCULATION", "PRIVATE PARTICIPANT PROFILE", "PRIVATE MEMORY", "PRIVATE RETRIEVAL INPUT", "PRIVATE FULL BODY"} {
		if strings.Contains(string(raw), forbidden) {
			t.Fatalf("failure audit leaked %s", forbidden)
		}
	}
	if len(doc.Blocks) != 1 || len(doc.Evidence) != 6 {
		t.Fatal("sanitizing the persisted audit mutated the generation document")
	}
}

func TestReportReadNeverPublishesPrivateCalibrationAudit(t *testing.T) {
	for _, status := range []string{"failed", "ready"} {
		t.Run(status, func(t *testing.T) {
			doc := reportFailureAudit(reportdoc.Document{Version: 1, Runtime: "harness", ModelCalls: 4, ToolCalls: 2, Evidence: []reportdoc.Evidence{{Tool: "report_calibration", Text: `{"issues":[{"quote":"PRIVATE REVIEW QUOTE","basis":"PRIVATE BASIS"}]}`}}})
			raw, _ := json.Marshal(doc)
			db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			row := sqlmock.NewRows(strings.Split(reportCols, ",")).AddRow(string(raw), status, 1, "ARaudit", "100", "bazi", "测试报告", "1", "公开问题", "{}", `[]`, 990, 0, status, "公开摘要", "公开正文", "", 0, "2026-10-28")
			mock.ExpectQuery(`SELECT `+reportSelectCols+` FROM ai_report WHERE id=? AND user_id=?`).WithArgs(int64(1), "100").WillReturnRows(row)
			mock.ExpectQuery(reportCashEntitlementSQL).WithArgs("ARaudit", "100", int64(990), int64(990)).WillReturnRows(sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(1))
			got, err := ReportGet(context.Background(), &svc.ServiceContext{DB: sqlx.NewSqlConnFromDB(db)}, "100", 1)
			if err != nil {
				t.Fatal(err)
			}
			if status == "failed" && (got.Unlocked || got.Document != nil || got.Content != "") {
				t.Fatal("failed report became readable because it had an audit or payment")
			}
			if status == "ready" && (!got.Unlocked || got.Document == nil || len(got.Document.Evidence) != 0) {
				t.Fatal("reader did not strip private evidence from an unlocked document")
			}
			public, _ := json.Marshal(got)
			for _, forbidden := range []string{"PRIVATE REVIEW QUOTE", "PRIVATE BASIS", "report_calibration", `"evidence"`, `"document_json"`} {
				if strings.Contains(string(public), forbidden) {
					t.Fatalf("public report leaked %s", forbidden)
				}
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

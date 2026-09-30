package model

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	"github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestRepeatedToolCallsIntegration(t *testing.T) {
	dsn := os.Getenv("AI_RUNTIME_TEST_DSN")
	if dsn == "" {
		t.Skip("requires disposable ai_runtime_test_* database")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil || !strings.HasPrefix(cfg.DBName, "ai_runtime_test_") {
		t.Fatal("requires disposable test database")
	}
	cfg.MultiStatements = true
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	source, err := os.ReadFile("../../../../../db/init.sql")
	if err != nil {
		t.Fatal(err)
	}
	start := strings.Index(string(source), "CREATE TABLE IF NOT EXISTS `ai_tool_call`")
	if start < 0 {
		t.Fatal("missing tool schema")
	}
	ddl := strings.SplitN(string(source)[start:], ";", 2)[0]
	migration, err := os.ReadFile("../../../../../scripts/db/20260930_ai_repeat_tool_calls.sql")
	if err != nil {
		t.Fatal(err)
	}
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "fresh-schema", true: "legacy-migration"}[legacy], func(t *testing.T) {
			_, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS ai_tool_call")
			t.Cleanup(func() { _, _ = db.ExecContext(ctx, "DROP TABLE IF EXISTS ai_tool_call") })
			create := ddl
			if legacy {
				create = strings.ReplaceAll(create, "KEY `idx_run_tool`", "UNIQUE KEY `uk_run_tool`")
			}
			if _, err = db.ExecContext(ctx, create); err != nil {
				t.Fatal(err)
			}
			model := NewRunModel(sqlx.NewSqlConnFromDB(db))
			first, err := model.StartTool(ctx, 1, "local", "search_knowledge", `{"query":"first"}`)
			if err != nil {
				t.Fatal(err)
			}
			if legacy {
				for i := 0; i < 2; i++ {
					if _, err = db.ExecContext(ctx, string(migration)); err != nil {
						t.Fatal(err)
					}
				}
			}
			second, err := model.StartTool(ctx, 1, "local", "search_knowledge", `{"query":"refined"}`)
			if err != nil || first == second {
				t.Fatalf("repeat rejected or overwritten: %d %d %v", first, second, err)
			}
			if err = model.CompleteTool(ctx, first, "first result", 10); err != nil {
				t.Fatal(err)
			}
			if err = model.CompleteTool(ctx, second, "second result", 20); err != nil {
				t.Fatal(err)
			}
			var count int
			if err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM ai_tool_call WHERE run_id=1 AND status='completed'").Scan(&count); err != nil || count != 2 {
				t.Fatalf("history lost: %d %v", count, err)
			}
		})
	}
}

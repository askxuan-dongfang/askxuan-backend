package model

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestDirectoryCascadeMySQL(t *testing.T) {
	dsn := os.Getenv("ASKXUAN_DIRECTORY_TEST_DSN")
	if dsn == "" {
		t.Skip("requires isolated directory fixture")
	}
	if dsn != "root@tcp(127.0.0.1:13319)/askxuan_temple?charset=utf8mb4" {
		t.Fatal("refusing non-isolated DB")
	}
	logx.Disable()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	exec := func(q string) {
		t.Helper()
		if _, err := db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	exec("CREATE TABLE IF NOT EXISTS directory_tag(code VARCHAR(32) PRIMARY KEY,tag_group VARCHAR(16),name VARCHAR(32),value VARCHAR(64),sort INT,status VARCHAR(16))")
	exec("DELETE FROM directory_tag WHERE code IN ('chan','quanzhen','test_tradition')")
	exec("INSERT INTO directory_tag(code,tag_group,name,value,sort,status) VALUES('chan','tradition','禅宗','chan',10,'enabled'),('quanzhen','tradition','全真派','quanzhen',20,'enabled')")
	migration, err := os.ReadFile("../../../../../scripts/db/20261019_directory_belief_cascade.sql")
	if err != nil {
		t.Fatal(err)
	}
	t.Run("migration can be repeated", func(t *testing.T) {
		connection, err := db.Conn(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer connection.Close()
		for i := 0; i < 2; i++ {
			for _, statement := range strings.Split(string(migration), ";") {
				if strings.TrimSpace(statement) != "" {
					if _, err := connection.ExecContext(context.Background(), statement); err != nil {
						t.Fatal(err)
					}
				}
			}
		}
	})
	m := NewDirectoryTagModel(sqlx.NewMysql(dsn))
	ctx := context.Background()
	t.Run("public parent metadata", func(t *testing.T) {
		rows, err := m.List(ctx, false)
		if err != nil || len(rows) != 2 || rows[0].BeliefCode != "han_buddhism" || rows[1].BeliefCode != "daoism" {
			t.Fatal(rows, err)
		}
	})
	t.Run("old admin rename preserves parent", func(t *testing.T) {
		p := DirectoryTag{Code: "chan", Group: "tradition", Name: "禅宗新名称", Value: "chan", Sort: 10, Status: "enabled"}
		if err := m.Save(ctx, &p, false); err != nil {
			t.Fatal(err)
		}
		rows, err := m.List(ctx, false)
		if err != nil || rows[0].BeliefCode != "han_buddhism" || rows[0].Name != p.Name {
			t.Fatal(rows, err)
		}
	})
	t.Run("new custom tag requires parent", func(t *testing.T) {
		p := DirectoryTag{Code: "test_tradition", Group: "tradition", Name: "测试信俗", Value: "test_tradition", Sort: 30, Status: "enabled"}
		if m.Save(ctx, &p, true) == nil {
			t.Fatal("missing parent accepted")
		}
		p.BeliefCode = "folk"
		if err := m.Save(ctx, &p, true); err != nil {
			t.Fatal(err)
		}
		p.BeliefCode = "daoism"
		if err := m.Save(ctx, &p, false); err != nil {
			t.Fatal(err)
		}
		rows, err := m.List(ctx, false)
		if err != nil || rows[len(rows)-1].BeliefCode != "daoism" {
			t.Fatal(rows, err)
		}
	})
	t.Run("disabled remains admin only", func(t *testing.T) {
		p := DirectoryTag{Code: "test_tradition", Group: "tradition", BeliefCode: "daoism", Name: "测试信俗", Value: "test_tradition", Sort: 30, Status: "disabled"}
		if err := m.Save(ctx, &p, false); err != nil {
			t.Fatal(err)
		}
		public, err := m.List(ctx, false)
		if err != nil || len(public) != 2 {
			t.Fatal(public, err)
		}
		admin, err := m.List(ctx, true)
		if err != nil || len(admin) != 3 {
			t.Fatal(admin, err)
		}
	})
}

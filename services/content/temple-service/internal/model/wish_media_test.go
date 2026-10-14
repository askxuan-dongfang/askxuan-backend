package model

import (
	"context"
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"os"
	"testing"
)

func TestTempleWishMediaMySQL(t *testing.T) {
	d := os.Getenv("ASKXUAN_WISH_TEST_DSN")
	if d == "" {
		t.Skip("requires isolated wish fixture")
	}
	if d != "root@tcp(127.0.0.1:13316)/askxuan_product?charset=utf8mb4" {
		t.Fatal("refusing non-isolated DB")
	}
	logx.Disable()
	dsn := "root@tcp(127.0.0.1:13316)/askxuan_temple?charset=utf8mb4"
	db, e := sql.Open("mysql", dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	ctx := context.Background()
	conn := sqlx.NewMysql(dsn)
	exec := func(q string) {
		t.Helper()
		if _, e := db.Exec(q); e != nil {
			t.Fatal(e)
		}
	}
	m := NewTempleServiceModel(conn)
	t.Run("inherit custom empty and reset", func(t *testing.T) {
		exec(`INSERT IGNORE INTO askxuan_product.service_intention_rule VALUES('S001','peace')`)
		if e := m.ReplaceIntentTags(ctx, 1, nil); e != nil {
			t.Fatal(e)
		}
		tags, e := m.FindIntentTags(ctx, 1)
		if e != nil || len(tags) != 1 || tags[0] != "peace" {
			t.Fatal(tags, e)
		}
		if e := m.ReplaceIntentTags(ctx, 1, []string{}); e != nil {
			t.Fatal(e)
		}
		tags, e = m.FindIntentTags(ctx, 1)
		if e != nil || len(tags) != 0 {
			t.Fatal(tags, e)
		}
		if e := m.ReplaceIntentTags(ctx, 1, []string{"diy"}); e == nil {
			t.Fatal("non-wish accepted")
		}
		if e := m.ReplaceIntentTags(ctx, 1, []string{"wealth"}); e == nil {
			t.Fatal("disabled wish accepted")
		}
		if e := m.ReplaceIntentTags(ctx, 1, nil); e != nil {
			t.Fatal(e)
		}
		tags, e = m.FindIntentTags(ctx, 1)
		if e != nil || len(tags) != 1 {
			t.Fatal(tags, e)
		}
	})
	t.Run("gallery identity and image provenance", func(t *testing.T) {
		im := NewTempleImageModel(conn)
		images, e := im.FindByTempleCode(ctx, "DT001")
		if e != nil || len(images) != 2 {
			t.Fatal(e, len(images))
		}
		if images[0].Caption == "" || images[0].Kind != "generated" {
			t.Fatal(images[0])
		}
		exec(`UPDATE temple SET name='new temple' WHERE code='DT001'`)
		images, e = im.FindByTempleCode(ctx, "DT001")
		if e != nil || len(images) != 0 {
			t.Fatal(e, len(images))
		}
	})
}

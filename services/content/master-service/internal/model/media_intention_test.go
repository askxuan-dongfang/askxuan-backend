package model

import (
	"context"
	"database/sql"
	"github.com/askxuan/master-service/internal/types"
	_ "github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"os"
	"testing"
)

func TestMasterMediaAndIntentMySQL(t *testing.T) {
	if os.Getenv("ASKXUAN_WISH_TEST_DSN") == "" {
		t.Skip("requires isolated wish fixture")
	}
	if os.Getenv("ASKXUAN_WISH_TEST_DSN") != "root@tcp(127.0.0.1:13316)/askxuan_product?charset=utf8mb4" {
		t.Fatal("refusing non-isolated DB")
	}
	logx.Disable()
	dsn := "root@tcp(127.0.0.1:13316)/askxuan_master?charset=utf8mb4"
	db, e := sql.Open("mysql", dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	exec := func(q string, args ...interface{}) {
		t.Helper()
		if _, e := db.Exec(q, args...); e != nil {
			t.Fatal(e)
		}
	}
	exec(`CREATE TABLE IF NOT EXISTS master_image(master_code VARCHAR(32),url VARCHAR(500),caption VARCHAR(255),kind VARCHAR(32),sort INT,name_guard VARCHAR(255),avatar_guard VARCHAR(500),PRIMARY KEY(master_code,url))`)
	exec(`DELETE FROM master_image`)
	exec(`UPDATE master SET avatar='/demo.jpg',dharma_name='fixture' WHERE code='platform'`)
	exec(`INSERT INTO master_image VALUES('platform','/demo.jpg','AI avatar','generated',1,'fixture','/demo.jpg'),('platform','/scene.jpg','AI scene','generated',2,'fixture','/demo.jpg')`)
	ctx := context.Background()
	m := NewMasterImageModel(sqlx.NewMysql(dsn))
	count := func(want int) {
		t.Helper()
		rows, e := m.FindPublic(ctx, "platform")
		if e != nil || len(rows) != want {
			t.Fatal(e, len(rows), want)
		}
	}
	t.Run("gallery and source labels", func(t *testing.T) {
		count(2)
		rows, _ := m.FindPublic(ctx, "platform")
		if rows[0].Kind != "generated" || rows[0].Caption == "" {
			t.Fatal(rows)
		}
	})
	t.Run("rename hides demo gallery", func(t *testing.T) {
		exec(`UPDATE master SET dharma_name='real person' WHERE code='platform'`)
		count(0)
		exec(`UPDATE master SET dharma_name='fixture' WHERE code='platform'`)
	})
	t.Run("operator avatar hides demo gallery", func(t *testing.T) {
		exec(`UPDATE master SET avatar='/operator.jpg' WHERE code='platform'`)
		count(0)
		exec(`UPDATE master SET avatar='/demo.jpg' WHERE code='platform'`)
	})
	t.Run("unpublished profile cannot expose gallery", func(t *testing.T) {
		exec(`UPDATE master SET shelf_status='off_shelf' WHERE code='platform'`)
		count(0)
		exec(`UPDATE master SET shelf_status='on_shelf' WHERE code='platform'`)
	})
	t.Run("price edit preserves custom empty and row identity", func(t *testing.T) {
		exec(`UPDATE master_service_tag SET intent_mode='custom' WHERE master_code='platform' AND service_code='S001'`)
		var before, after int
		var mode string
		db.QueryRow(`SELECT id FROM master_service_tag WHERE master_code='platform' AND service_code='S001'`).Scan(&before)
		model := NewMasterModel(sqlx.NewMysql(dsn))
		if e := model.ReplaceServiceTags(ctx, "platform", []types.MasterServiceTagItem{{ServiceCode: "S001", Price: 49}, {ServiceCode: "S004", Price: 99}}); e != nil {
			t.Fatal(e)
		}
		db.QueryRow(`SELECT id,intent_mode FROM master_service_tag WHERE master_code='platform' AND service_code='S001'`).Scan(&after, &mode)
		if before != after || mode != "custom" {
			t.Fatal(before, after, mode)
		}
		db.QueryRow(`SELECT intent_mode FROM master_service_tag WHERE master_code='platform' AND service_code='S004'`).Scan(&mode)
		if mode != "inherit" {
			t.Fatal(mode)
		}
		var count int
		db.QueryRow(`SELECT COUNT(*) FROM master_service_tag WHERE master_code='platform'`).Scan(&count)
		if count != 2 {
			t.Fatal(count)
		}
		if e := model.ReplaceServiceTags(ctx, "platform", nil); e != nil {
			t.Fatal(e)
		}
		db.QueryRow(`SELECT COUNT(*) FROM master_service_tag WHERE master_code='platform'`).Scan(&count)
		if count != 0 {
			t.Fatal(count)
		}
	})
}

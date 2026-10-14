package model

import (
	"context"
	"database/sql"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"os"
	"strings"
	"testing"
)

func TestBindingValidation(t *testing.T) {
	for _, r := range []ServiceIntentionUpdate{
		{Kind: "temple", ID: "-1", Service: "S001"}, {Kind: "master", ID: "", Service: "S001"},
		{Kind: "master", ID: "M1", Service: "S001", Tags: []string{"diy"}},
		{Kind: "master", ID: "M1", Service: "S001", Tags: []string{"peace", "peace"}},
		{Kind: "master", ID: "M1", Service: "S001", Mode: "inherit", Tags: []string{"peace"}},
		{Kind: "rule", Service: "S001", Mode: "inherit"}, {Kind: "rule", Service: "'; DELETE", Mode: "custom"},
		{Kind: "master", ID: "M1", Service: "S001", Mode: "invalid"},
	} {
		if r.Validate() == nil {
			t.Fatalf("accepted invalid update: %+v", r)
		}
	}
	for _, r := range []ServiceIntentionUpdate{{Kind: "temple", ID: "1", Service: "S001"}, {Kind: "master", ID: "M1", Service: "S001", Mode: "inherit"}, {Kind: "rule", Service: "S001", Tags: []string{"peace"}}} {
		if e := r.Validate(); e != nil {
			t.Fatal(e)
		}
	}
}

// Only the disposable Docker instance created by scripts/test-wish-mysql.sh is accepted.
func TestIntentionMySQL(t *testing.T) {
	logx.Disable()
	dsn := os.Getenv("ASKXUAN_WISH_TEST_DSN")
	if dsn == "" {
		t.Skip("run scripts/test-wish-mysql.sh for isolated MySQL tests")
	}
	if dsn != "root@tcp(127.0.0.1:13316)/askxuan_product?charset=utf8mb4" {
		t.Fatal("refusing non-isolated DSN")
	}
	db, e := sql.Open("mysql", strings.Replace(dsn, "/askxuan_product?", "/?", 1))
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	exec := func(q string, args ...interface{}) {
		t.Helper()
		if _, e := db.Exec(q, args...); e != nil {
			t.Fatal(q, e)
		}
	}
	for _, schema := range []string{"askxuan_product", "askxuan_temple", "askxuan_master"} {
		exec("CREATE DATABASE IF NOT EXISTS " + schema + " CHARACTER SET utf8mb4")
	}
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS askxuan_product.intent_tag(code varchar(32) PRIMARY KEY,status varchar(32))`,
		`CREATE TABLE IF NOT EXISTS askxuan_product.service_intention_rule(service_code varchar(32),tag_code varchar(32),PRIMARY KEY(service_code,tag_code))`,
		`CREATE TABLE IF NOT EXISTS askxuan_product.master_service_intention(master_code varchar(32),service_code varchar(32),tag_code varchar(32),PRIMARY KEY(master_code,service_code,tag_code))`,
		`CREATE TABLE IF NOT EXISTS askxuan_temple.temple_service_intent_tag(temple_service_id bigint,tag_code varchar(32),PRIMARY KEY(temple_service_id,tag_code))`,
		`CREATE TABLE IF NOT EXISTS askxuan_temple.service_type(code varchar(32) PRIMARY KEY,name varchar(128),status varchar(32))`,
		`CREATE TABLE IF NOT EXISTS askxuan_temple.temple(code varchar(32) PRIMARY KEY,name varchar(128),status varchar(32),description text,cover_image text)`,
		`CREATE TABLE IF NOT EXISTS askxuan_temple.temple_service(id bigint PRIMARY KEY,temple_code varchar(32),service_code varchar(32),service_name varchar(128),price decimal(10,2),status varchar(32),intent_mode varchar(16) DEFAULT 'inherit',update_time timestamp DEFAULT CURRENT_TIMESTAMP,UNIQUE(temple_code,service_code))`,
		`CREATE TABLE IF NOT EXISTS askxuan_master.master(code varchar(32) PRIMARY KEY,dharma_name varchar(128),temple_code varchar(32),type varchar(64),sect varchar(64),avatar text,auth_status varchar(32),shelf_status varchar(32),platform_status varchar(32),manage_by varchar(32))`,
		`CREATE TABLE IF NOT EXISTS askxuan_master.master_service_tag(id bigint AUTO_INCREMENT PRIMARY KEY,master_code varchar(32),service_code varchar(32),price decimal(10,2),status varchar(32),intent_mode varchar(16) DEFAULT 'inherit',update_time timestamp DEFAULT CURRENT_TIMESTAMP,UNIQUE(master_code,service_code))`,
	}
	for _, q := range stmts {
		exec(q)
	}
	for _, table := range []string{"askxuan_product.intent_tag", "askxuan_product.service_intention_rule", "askxuan_product.master_service_intention", "askxuan_temple.temple_service_intent_tag", "askxuan_temple.service_type", "askxuan_temple.temple", "askxuan_temple.temple_service", "askxuan_master.master", "askxuan_master.master_service_tag"} {
		exec("DELETE FROM " + table)
	}
	exec(`INSERT INTO askxuan_product.intent_tag VALUES('peace','enabled'),('wealth','enabled')`)
	exec(`INSERT INTO askxuan_product.service_intention_rule VALUES('S001','peace'),('S002','peace'),('S003','peace')`)
	exec(`INSERT INTO askxuan_temple.service_type VALUES('S001','祈福','enabled'),('S002','供灯','enabled'),('S003','上香','enabled')`)
	exec(`INSERT INTO askxuan_temple.temple VALUES('T1','演示寺','正常','','')`)
	exec(`INSERT INTO askxuan_temple.temple_service(id,temple_code,service_code,service_name,price,status) VALUES(1,'T1','S001','祈福',200,'on_shelf'),(2,'T1','S002','供灯',80,'on_shelf')`)
	for _, track := range []string{"platform", "temple"} {
		tc := ""
		if track == "temple" {
			tc = "T1"
		}
		exec(`INSERT INTO askxuan_master.master VALUES(?,?,?,'演示','演示','','已认证','on_shelf','normal',?)`, track, track, tc, track)
		for _, sc := range []string{"S001", "S002", "S003"} {
			exec(`INSERT INTO askxuan_master.master_service_tag(master_code,service_code,price,status) VALUES(?,?,39,'enabled')`, track, sc)
		}
	}
	conn := sqlx.NewMysql(dsn)
	m := NewIntentionModel(conn)
	ctx := context.Background()
	count := func(code string, want int64) {
		t.Helper()
		rows, total, e := m.FindResources(ctx, code, 1, 100)
		if e != nil || total != want || int64(len(rows)) != want {
			t.Fatalf("%s want=%d total=%d rows=%d err=%v", code, want, total, len(rows), e)
		}
	}
	update := func(r ServiceIntentionUpdate) {
		t.Helper()
		if e := UpdateServiceIntentions(ctx, conn, r); e != nil {
			t.Fatal(e)
		}
	}
	t.Run("inherit intersection and total price", func(t *testing.T) {
		count("peace", 7)
		rows, _, _ := m.FindResources(ctx, "peace", 1, 100)
		for _, r := range rows {
			if r.SourceId == "temple" && r.ServiceCode == "S001" && r.Price != 239 {
				t.Fatal("missing temple fee", r.Price)
			}
			if r.SourceId == "temple" && r.ServiceCode == "S003" {
				t.Fatal("unavailable service exposed")
			}
		}
	})
	t.Run("explicit empty and reset", func(t *testing.T) {
		update(ServiceIntentionUpdate{Kind: "temple", ID: "1", Service: "S001", Mode: "custom"})
		count("peace", 6)
		update(ServiceIntentionUpdate{Kind: "temple", ID: "1", Service: "S001", Mode: "inherit"})
		count("peace", 7)
	})
	t.Run("manual override is not replaced by defaults", func(t *testing.T) {
		update(ServiceIntentionUpdate{Kind: "master", ID: "platform", Service: "S001", Tags: []string{"wealth"}})
		count("peace", 6)
		count("wealth", 1)
		update(ServiceIntentionUpdate{Kind: "rule", Service: "S001", Tags: []string{"wealth"}})
		count("peace", 4)
		count("wealth", 3)
		update(ServiceIntentionUpdate{Kind: "rule", Service: "S001", Tags: []string{"peace"}})
		update(ServiceIntentionUpdate{Kind: "master", ID: "platform", Service: "S001", Mode: "inherit"})
		count("peace", 7)
	})
	t.Run("validation and missing service leave existing data intact", func(t *testing.T) {
		exec(`UPDATE askxuan_product.intent_tag SET status='disabled' WHERE code='wealth'`)
		if UpdateServiceIntentions(ctx, conn, ServiceIntentionUpdate{Kind: "master", ID: "platform", Service: "S001", Tags: []string{"wealth"}}) == nil {
			t.Fatal("accepted disabled wish")
		}
		if UpdateServiceIntentions(ctx, conn, ServiceIntentionUpdate{Kind: "master", ID: "missing", Service: "S001"}) == nil {
			t.Fatal("accepted missing service")
		}
		count("peace", 7)
	})
	t.Run("database failure rolls back mode and bindings together", func(t *testing.T) {
		exec(`UPDATE askxuan_product.intent_tag SET status='enabled' WHERE code='wealth'`)
		exec(`CREATE TRIGGER askxuan_product.reject_fixture_binding BEFORE INSERT ON askxuan_product.master_service_intention FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='fixture failure'`)
		err := UpdateServiceIntentions(ctx, conn, ServiceIntentionUpdate{Kind: "master", ID: "platform", Service: "S001", Tags: []string{"wealth"}})
		exec(`DROP TRIGGER askxuan_product.reject_fixture_binding`)
		if err == nil {
			t.Fatal("database failure ignored")
		}
		count("peace", 7)
		count("wealth", 0)
		exec(`UPDATE askxuan_product.intent_tag SET status='disabled' WHERE code='wealth'`)
	})
	t.Run("disabled service and provider are filtered", func(t *testing.T) {
		exec(`UPDATE askxuan_temple.service_type SET status='disabled' WHERE code='S001'`)
		count("peace", 4)
		exec(`UPDATE askxuan_temple.service_type SET status='enabled'`)
		exec(`UPDATE askxuan_temple.temple SET status='下架'`)
		count("peace", 3)
		exec(`UPDATE askxuan_temple.temple SET status='正常'`)
		exec(`UPDATE askxuan_master.master SET shelf_status='off_shelf' WHERE code='platform'`)
		count("peace", 4)
		exec(`UPDATE askxuan_master.master SET shelf_status='on_shelf'`)
	})
	t.Run("stable pagination never loses services with equal timestamps", func(t *testing.T) {
		seen := map[string]bool{}
		for p := 1; p <= 4; p++ {
			rows, total, e := m.FindResources(ctx, "peace", p, 2)
			if e != nil || total != 7 {
				t.Fatal(e, total)
			}
			for _, r := range rows {
				k := fmt.Sprint(r.ResourceType, ":", r.SourceId, ":", r.ServiceCode)
				if seen[k] {
					t.Fatal("duplicate", k)
				}
				seen[k] = true
			}
		}
		if len(seen) != 7 {
			t.Fatal(seen)
		}
	})
	t.Run("admin list resolves effective tags and inheritance source", func(t *testing.T) {
		rows, e := ListServiceIntentions(ctx, conn)
		if e != nil || len(rows) != 11 {
			t.Fatal(e, len(rows))
		}
		for _, r := range rows {
			if r.Kind != "rule" && r.Mode != "inherit" {
				t.Fatal(r)
			}
		}
	})
}

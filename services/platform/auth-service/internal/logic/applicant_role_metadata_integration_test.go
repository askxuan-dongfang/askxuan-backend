package logic

import (
	"context"
	"database/sql"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

func applicantRoleSQL(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile("../../../../../" + path)
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, line := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "--") {
			lines = append(lines, line)
		}
	}
	var statements []string
	for _, statement := range strings.Split(strings.Join(lines, "\n"), ";") {
		if statement = strings.TrimSpace(statement); statement != "" {
			statements = append(statements, statement)
		}
	}
	return statements
}

func TestApplicantRoleSQLCharset(t *testing.T) {
	for _, path := range []string{"scripts/db/20260915_dual_track_onboarding.sql", "scripts/dev/email-auth/onboarding-schema.sql", "scripts/db/20261023_applicant_role_metadata.sql"} {
		if statements := applicantRoleSQL(t, path); len(statements) == 0 || statements[0] != "SET NAMES utf8mb4" {
			t.Errorf("%s must set the session charset before writing Chinese metadata", path)
		}
	}
	b, err := os.ReadFile("../../../../../scripts/dev/docker-up-all.sh")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.Contains(line, "docker exec -i askxuan-mysql mysql ") && !strings.Contains(line, "--default-character-set=utf8mb4") {
			t.Error("local initialization and migration clients must explicitly use utf8mb4")
		}
	}
}

func TestApplicantRoleMetadataMySQL(t *testing.T) {
	dsn := os.Getenv("ACCOUNT_SYSTEM_TEST_DSN")
	if dsn == "" || os.Getenv("ACCOUNT_SYSTEM_TEST_ISOLATED") != "1" {
		t.Skip("requires the disposable account-system MySQL instance")
	}
	cfg, err := mysql.ParseDSN(dsn)
	if err != nil || cfg.DBName != "askxuan_account_test" || cfg.Net != "tcp" || !strings.HasPrefix(cfg.Addr, "127.0.0.1:") {
		t.Fatal("refusing database outside the isolated account-system instance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal("cannot open isolated MySQL")
	}
	defer db.Close()
	const schema = "askxuan_applicant_role_test"
	if _, err := db.ExecContext(ctx, "CREATE DATABASE IF NOT EXISTS "+schema+" CHARACTER SET utf8mb4"); err != nil {
		t.Fatal(err)
	}
	cfg.DBName = schema
	fixture, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		t.Fatal("cannot open applicant-role fixture schema")
	}
	defer fixture.Close()
	for _, ddl := range []string{
		`CREATE TABLE IF NOT EXISTS role(id BIGINT AUTO_INCREMENT PRIMARY KEY,name VARCHAR(64) NOT NULL,code VARCHAR(64) NOT NULL UNIQUE,description VARCHAR(255) NOT NULL DEFAULT '',authz_scope VARCHAR(16) NOT NULL DEFAULT 'platform',authz_version BIGINT NOT NULL DEFAULT 0,create_time DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
		`CREATE TABLE IF NOT EXISTS admin_account(id BIGINT PRIMARY KEY,role_id BIGINT NOT NULL,temple_id VARCHAR(16) NOT NULL,master_id VARCHAR(16) NOT NULL,status VARCHAR(16) NOT NULL,authz_version BIGINT NOT NULL) ENGINE=InnoDB`,
		`CREATE TABLE IF NOT EXISTS role_permission(role_id BIGINT NOT NULL,permission_id BIGINT NOT NULL,PRIMARY KEY(role_id,permission_id)) ENGINE=InnoDB`,
	} {
		if _, err := fixture.ExecContext(ctx, ddl); err != nil {
			t.Fatal(err)
		}
	}
	for _, table := range []string{"role", "admin_account", "role_permission"} {
		var count int
		if err := fixture.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil || count != 0 {
			t.Fatal("applicant-role fixture must be empty; refusing to overwrite data")
		}
	}
	tx, err := fixture.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	exec := func(query string) {
		t.Helper()
		if _, err := tx.ExecContext(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	seed := applicantRoleSQL(t, "scripts/db/20260915_dual_track_onboarding.sql")[:2]
	apply := func(statements []string) {
		for _, statement := range statements {
			exec(strings.ReplaceAll(statement, "askxuan_auth.", schema+"."))
		}
	}
	repair := applicantRoleSQL(t, "scripts/db/20261023_applicant_role_metadata.sql")
	metadata := func() map[string][2]string {
		t.Helper()
		rows, err := tx.QueryContext(ctx, "SELECT code,name,description FROM role")
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		out := map[string][2]string{}
		for rows.Next() {
			var code, name, description string
			if err := rows.Scan(&code, &name, &description); err != nil {
				t.Fatal(err)
			}
			out[code] = [2]string{name, description}
		}
		if err := rows.Err(); err != nil {
			t.Fatal(err)
		}
		return out
	}
	authorization := func() []string {
		t.Helper()
		var out []string
		for _, query := range []string{"SELECT CONCAT_WS('|',id,code,authz_scope,authz_version,create_time) FROM role ORDER BY id", "SELECT CONCAT_WS('|',id,role_id,temple_id,master_id,status,authz_version) FROM admin_account ORDER BY id", "SELECT CONCAT_WS('|',role_id,permission_id) FROM role_permission ORDER BY role_id,permission_id"} {
			rows, err := tx.QueryContext(ctx, query)
			if err != nil {
				t.Fatal(err)
			}
			for rows.Next() {
				var value string
				if err := rows.Scan(&value); err != nil {
					t.Fatal(err)
				}
				out = append(out, value)
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			rows.Close()
		}
		return out
	}
	// Reproduce the historical write using the actual seed, without its new charset preamble.
	exec("SET NAMES latin1")
	apply(seed[1:])
	exec("SET NAMES utf8mb4")
	broken := metadata()
	exec(`INSERT INTO role(name,code,description,authz_scope,authz_version) SELECT name,'custom_applicant',description,'platform',9 FROM role WHERE code='master_applicant'`)
	exec(`INSERT INTO role(name,code,description,authz_scope,authz_version) VALUES('原寺院管理员','temple_admin','原有说明','temple',11),('自定义岗位','custom_valid','保留说明','platform',12)`)
	exec(`UPDATE role SET authz_scope=CASE code WHEN 'master_applicant' THEN 'master' ELSE 'temple' END,authz_version=7 WHERE code IN ('master_applicant','temple_applicant')`)
	exec(`INSERT INTO admin_account SELECT id,id,'T-FIXTURE','M-FIXTURE','enabled',5 FROM role`)
	exec(`INSERT INTO role_permission SELECT id,17 FROM role`)
	before := metadata()
	authzBefore := authorization()
	apply(repair)
	want := map[string][2]string{"master_applicant": {"大师认证申请人", "仅可管理本人独立大师认证申请"}, "temple_applicant": {"寺院入驻申请人", "仅可管理本人寺院入驻申请"}}
	for code, pair := range want {
		if broken[code] == pair || metadata()[code] != pair {
			t.Fatalf("latin1 corruption was not reproduced and repaired for %s", code)
		}
	}
	for _, code := range []string{"custom_applicant", "custom_valid", "temple_admin"} {
		if metadata()[code] != before[code] {
			t.Errorf("unrelated role %s changed", code)
		}
	}
	fixed := metadata()
	apply(repair)
	if !reflect.DeepEqual(fixed, metadata()) || !reflect.DeepEqual(authzBefore, authorization()) {
		t.Fatal("repeat repair changed metadata or authorization data")
	}
	exec(`UPDATE role SET name='定制名称',description='定制说明' WHERE code IN ('master_applicant','temple_applicant')`)
	custom := metadata()
	apply(repair)
	apply(seed)
	if !reflect.DeepEqual(custom, metadata()) {
		t.Fatal("repair or repeated seed overwrote valid customized metadata")
	}
	exec("SET NAMES latin1")
	exec(`UPDATE role SET name='大师认证申请人' WHERE code='master_applicant'`)
	exec(`UPDATE role SET description='仅可管理本人寺院入驻申请' WHERE code='temple_applicant'`)
	apply(repair)
	if got := metadata(); got["master_applicant"] != [2]string{"大师认证申请人", "定制说明"} || got["temple_applicant"] != [2]string{"定制名称", "仅可管理本人寺院入驻申请"} || !reflect.DeepEqual(authzBefore, authorization()) {
		t.Fatal("repair must match each corrupt field independently and preserve all authorization data")
	}
	exec(`DELETE FROM role WHERE code IN ('master_applicant','temple_applicant')`)
	withoutApplicants := metadata()
	apply(repair)
	if !reflect.DeepEqual(withoutApplicants, metadata()) {
		t.Fatal("repair inserted a missing role")
	}
	apply(seed)
	for code, pair := range want {
		if metadata()[code] != pair {
			t.Errorf("new seed did not initialize UTF-8 metadata for %s", code)
		}
	}
}

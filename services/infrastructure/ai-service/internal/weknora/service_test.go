package weknora

import (
	"context"
	"encoding/json"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const kbID = "22222222-2222-4222-8222-222222222222"
const docID = "33333333-3333-4333-8333-333333333333"

var baseCols = []string{"id", "name", "description", "enabled", "revision"}
var docCols = []string{"id", "base_id", "source", "enabled", "revision"}

func TestRetrievalScopeAndRevocation(t *testing.T) {
	for _, revoked := range []bool{false, true} {
		t.Run(map[bool]string{false: "scoped", true: "revoked-during-search"}[revoked], func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("X-API-Key") != "private" || r.URL.Path != "/api/v1/knowledge-bases/"+kbID+"/hybrid-search" {
					t.Error("incorrect credential/path")
				}
				var q struct {
					IDs  []string `json:"knowledge_ids"`
					Skip bool     `json:"skip_context_enrichment"`
				}
				_ = json.NewDecoder(r.Body).Decode(&q)
				if len(q.IDs) != 1 || q.IDs[0] != docID || !q.Skip {
					t.Error("retrieval broadened")
				}
				json.NewEncoder(w).Encode(map[string]any{"success": true, "data": []map[string]any{{"id": "chunk-ok", "knowledge_id": docID, "content": "测试原文", "score": 0.9}, {"id": "foreign", "knowledge_id": "outside", "content": "private"}}})
			}))
			defer server.Close()
			c, _ := New(server.URL, "private", "embedding")
			db, m, _ := sqlmock.New()
			defer db.Close()
			s := &Service{DB: sqlx.NewSqlConnFromDB(db), Client: c}
			m.ExpectQuery("SELECT id,name,description,enabled,revision FROM ai_knowledge_base ORDER").WillReturnRows(sqlmock.NewRows(baseCols).AddRow(kbID, "测试库", "", true, 1))
			m.ExpectQuery("SELECT id,base_id,source,enabled,revision").WithArgs(kbID).WillReturnRows(sqlmock.NewRows(docCols).AddRow(docID, kbID, "测试出处", true, 2).AddRow("disabled", kbID, "未审核", false, 1))
			m.ExpectQuery("SELECT id,name,description,enabled,revision FROM ai_knowledge_base WHERE").WithArgs(kbID).WillReturnRows(sqlmock.NewRows(baseCols).AddRow(kbID, "测试库", "", true, 1))
			m.ExpectQuery("SELECT id,base_id,source,enabled,revision").WithArgs(kbID).WillReturnRows(sqlmock.NewRows(docCols).AddRow(docID, kbID, "测试出处", !revoked, 2))
			out, e := s.Search(context.Background(), "查询", []string{kbID})
			if e != nil {
				t.Fatal(e)
			}
			if revoked && len(out.Hits) != 0 {
				t.Fatal("revoked content leaked")
			}
			if !revoked && (len(out.Hits) != 1 || out.Hits[0].Source != "测试出处" || len(out.Hits[0].Hash) != 64) {
				t.Fatalf("unexpected citations: %+v", out)
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestEmptyScopeNeverSearches(t *testing.T) {
	out, e := (&Service{}).Search(context.Background(), "问题", nil)
	if e != nil || len(out.Hits) != 0 {
		t.Fatal("empty scope is not deny-all")
	}
}
func TestClientRejectsRedirectAndRedactsErrors(t *testing.T) {
	for _, code := range []int{302, 401, 500} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Location", "http://example.com")
			w.WriteHeader(code)
			w.Write([]byte("secret-upstream-diagnostic"))
		}))
		c, _ := New(srv.URL, "secret-key", "e")
		_, e := c.json(context.Background(), "GET", "/knowledge-bases", nil)
		srv.Close()
		if e == nil || strings.Contains(e.Error(), "secret") {
			t.Fatal("upstream failure leaked or ignored")
		}
	}
}
func TestUnregisteredDocumentNeverReachesUpstream(t *testing.T) {
	db, m, _ := sqlmock.New()
	defer db.Close()
	s := &Service{DB: sqlx.NewSqlConnFromDB(db)}
	m.ExpectQuery("SELECT id,name,description,enabled,revision").WithArgs(kbID).WillReturnRows(sqlmock.NewRows(baseCols).AddRow(kbID, "测试库", "", true, 1))
	m.ExpectQuery("SELECT id,base_id,source,enabled,revision").WithArgs(kbID).WillReturnRows(sqlmock.NewRows(docCols))
	if _, e := s.document(context.Background(), kbID, docID); e != ErrInput {
		t.Fatal(e)
	}
	if e := m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}

func TestMultipleBasesShareOneRankingAndRemainScoped(t *testing.T) {
	const second = "44444444-4444-4444-8444-444444444444"
	const secondDoc = "55555555-5555-4555-8555-555555555555"
	const disabled = "66666666-6666-4666-8666-666666666666"
	for _, revoke := range []bool{false, true} {
		t.Run(map[bool]string{false: "global-ranking", true: "base-revoked"}[revoke], func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				var q struct {
					Bases []string `json:"knowledge_base_ids"`
					Docs  []string `json:"knowledge_ids"`
				}
				if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
					t.Error(err)
				}
				if len(q.Bases) != 2 || q.Bases[0] != kbID || q.Bases[1] != second || len(q.Docs) != 2 || q.Docs[0] != docID || q.Docs[1] != secondDoc {
					t.Errorf("unbounded scope: %+v", q)
				}
				json.NewEncoder(w).Encode(map[string]any{"success": true, "data": []map[string]any{
					{"id": "specific", "knowledge_id": secondDoc, "content": "九二見龍在田", "score": .03},
					{"id": "generic", "knowledge_id": docID, "content": "一般导读", "score": .01},
					{"id": "newly-approved", "knowledge_id": "new-doc", "content": "not in request", "score": 1},
					{"id": "foreign", "knowledge_id": "foreign", "content": "private", "score": 1},
				}})
			}))
			defer server.Close()
			c, _ := New(server.URL, "private", "embedding")
			db, m, _ := sqlmock.New()
			defer db.Close()
			s := &Service{DB: sqlx.NewSqlConnFromDB(db), Client: c}
			m.ExpectQuery("SELECT id,name,description,enabled,revision FROM ai_knowledge_base ORDER").WillReturnRows(sqlmock.NewRows(baseCols).AddRow(kbID, "导读", "", true, 1).AddRow(second, "原文", "", true, 1).AddRow(disabled, "停用", "", false, 1))
			for _, pair := range [][2]string{{kbID, docID}, {second, secondDoc}} {
				m.ExpectQuery("SELECT id,base_id,source,enabled,revision").WithArgs(pair[0]).WillReturnRows(sqlmock.NewRows(docCols).AddRow(pair[1], pair[0], "source", true, 1))
			}
			for _, pair := range [][2]string{{kbID, docID}, {second, secondDoc}} {
				enabled := !revoke || pair[0] != second
				m.ExpectQuery("SELECT id,name,description,enabled,revision FROM ai_knowledge_base WHERE").WithArgs(pair[0]).WillReturnRows(sqlmock.NewRows(baseCols).AddRow(pair[0], "name", "", enabled, 1))
				if enabled {
					m.ExpectQuery("SELECT id,base_id,source,enabled,revision").WithArgs(pair[0]).WillReturnRows(sqlmock.NewRows(docCols).AddRow(pair[1], pair[0], "source", true, 1).AddRow("new-doc", pair[0], "new", true, 1))
				}
			}
			result, err := s.Search(context.Background(), "乾 九二", []string{kbID, second, disabled})
			if err != nil {
				t.Fatal(err)
			}
			if calls != 1 {
				t.Fatalf("per-base rank fusion returned: %d calls", calls)
			}
			if revoke {
				if len(result.Hits) != 1 || result.Hits[0].ID != "weknora:generic" {
					t.Fatalf("revoked base leaked: %+v", result)
				}
			} else if len(result.Hits) != 2 || result.Hits[0].ID != "weknora:specific" {
				t.Fatalf("lost global ranking: %+v", result)
			}
			if err = m.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

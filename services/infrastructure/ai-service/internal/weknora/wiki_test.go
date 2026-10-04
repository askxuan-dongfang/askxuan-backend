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

func TestWikiRejectsUnsafePathBeforeAccess(t *testing.T) {
	for _, slug := range []string{"../models", "/models", "concept/../models", "a%2fb", "a?x=1", "a#x", "a\\b", "a\n"} {
		if _, e := (&Service{}).WikiRead(context.Background(), kbID, "page", slug, "", 1); e != ErrInput {
			t.Fatalf("accepted %q: %v", slug, e)
		}
	}
	if _, e := (&Service{}).WikiRead(context.Background(), kbID, "models", "", "", 1); e != ErrInput {
		t.Fatal("arbitrary API accepted")
	}
}
func TestWikiScopedRawResponseAndRedaction(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-API-Key") != "private" {
			t.Error("missing authentication")
		}
		if r.URL.Path == "/api/v1/knowledge-bases/"+kbID {
			json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{"indexing_strategy": map[string]bool{"wiki_enabled": true}, "storage_config": "secret"}})
			return
		}
		if r.URL.Path != "/api/v1/knowledgebase/"+kbID+"/wiki/pages/concept/五行" {
			t.Error(r.URL.Path)
		}
		w.Write([]byte(`{"title":"五行","slug":"concept/五行","content":"原文","source_refs":["document|出处"],"tenant_id":99,"api_key":"secret"}`))
	}))
	defer srv.Close()
	c, _ := New(srv.URL, "private", "embedding")
	db, m, _ := sqlmock.New()
	defer db.Close()
	s := &Service{DB: sqlx.NewSqlConnFromDB(db), Client: c}
	m.ExpectQuery("SELECT id,name,description,enabled,revision").WithArgs(kbID).WillReturnRows(sqlmock.NewRows(baseCols).AddRow(kbID, "库", "", true, 1))
	m.ExpectQuery("SELECT id,base_id,source,enabled,revision").WithArgs(kbID).WillReturnRows(sqlmock.NewRows(docCols).AddRow("document", kbID, "公开出处", true, 1))
	v, e := s.WikiRead(context.Background(), kbID, "page", "concept/五行", "", 1)
	if e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(v)
	if strings.Contains(string(b), "secret") || strings.Contains(string(b), "tenant_id") || !strings.Contains(string(b), "原文") {
		t.Fatal(string(b))
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
func TestWikiUnknownBaseDoesNotReachEngine(t *testing.T) {
	db, m, _ := sqlmock.New()
	defer db.Close()
	s := &Service{DB: sqlx.NewSqlConnFromDB(db)}
	m.ExpectQuery("SELECT id,name,description,enabled,revision").WithArgs(kbID).WillReturnRows(sqlmock.NewRows(baseCols))
	if _, e := s.WikiRead(context.Background(), kbID, "stats", "", "", 1); e != ErrInput {
		t.Fatal(e)
	}
	if e := m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}

func TestWikiSettingsPreserveParserAndRejectStaleRevision(t *testing.T) {
	for _, stale := range []bool{false, true} {
		t.Run(map[bool]string{false: "preserve", true: "stale"}[stale], func(t *testing.T) {
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/v1/models" {
					w.Write([]byte(`{"success":true,"data":[{"id":"model-a","name":"test","type":"KnowledgeQA","parameters":{"api_key":"secret"}}]}`))
					return
				}
				calls++
				if r.Method == "GET" {
					w.Write([]byte(`{"success":true,"data":{"name":"原库","description":"保持","chunking_config":{"chunk_size":450,"chunk_overlap":60},"image_processing_config":{"model_id":"existing"},"indexing_strategy":{"vector_enabled":true,"keyword_enabled":true,"wiki_enabled":false,"graph_enabled":false}}}`))
					return
				}
				var in struct {
					Name   string `json:"name"`
					Config struct {
						Chunks struct {
							Size int `json:"chunk_size"`
						} `json:"chunking_config"`
						Strategy map[string]bool `json:"indexing_strategy"`
					} `json:"config"`
				}
				if e := json.NewDecoder(r.Body).Decode(&in); e != nil {
					t.Fatal(e)
				}
				if in.Name != "原库" || in.Config.Chunks.Size != 450 || !in.Config.Strategy["vector_enabled"] || !in.Config.Strategy["keyword_enabled"] || !in.Config.Strategy["wiki_enabled"] || in.Config.Strategy["graph_enabled"] {
					t.Errorf("destructive settings update: %+v", in)
				}
				w.Write([]byte(`{"success":true,"data":{}}`))
			}))
			defer srv.Close()
			c, _ := New(srv.URL, "private", "embedding")
			db, m, _ := sqlmock.New()
			defer db.Close()
			s := &Service{DB: sqlx.NewSqlConnFromDB(db), Client: c}
			m.ExpectQuery("SELECT id,name,description,enabled,revision").WithArgs(kbID).WillReturnRows(sqlmock.NewRows(baseCols).AddRow(kbID, "库", "", true, 2))
			m.ExpectBegin()
			m.ExpectQuery("SELECT id,name,description,enabled,revision.*FOR UPDATE").WithArgs(kbID).WillReturnRows(sqlmock.NewRows(baseCols).AddRow(kbID, "库", "", true, 2))
			rev := int64(2)
			if stale {
				rev = 1
				m.ExpectRollback()
			} else {
				m.ExpectExec("UPDATE ai_knowledge_base SET revision").WithArgs(kbID).WillReturnResult(sqlmock.NewResult(0, 1))
				m.ExpectExec("INSERT INTO ai_knowledge_audit").WithArgs("admin", "configure_wiki", kbID).WillReturnResult(sqlmock.NewResult(1, 1))
				m.ExpectCommit()
			}
			e := s.ConfigureWiki(context.Background(), kbID, "admin", WikiSettings{Enabled: true, Model: "model-a", Revision: rev})
			if stale {
				if e != ErrConflict || calls != 0 {
					t.Fatalf("stale update reached engine: %v %d", e, calls)
				}
			} else if e != nil || calls != 2 {
				t.Fatalf("settings update failed: %v %d", e, calls)
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}

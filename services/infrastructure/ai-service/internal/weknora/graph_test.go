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

func TestEntityGraphIsScopedAndRevocationWins(t *testing.T) {
	for _, revoked := range []bool{false, true} {
		t.Run(map[bool]string{false: "approved", true: "revoked"}[revoked], func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				u, p, ok := r.BasicAuth()
				if !ok || u != "neo4j" || p != "private" {
					t.Error("auth")
				}
				var body struct {
					Statements []struct {
						Statement  string `json:"statement"`
						Parameters struct {
							Docs []string `json:"docs"`
							Q    string   `json:"q"`
						} `json:"parameters"`
					} `json:"statements"`
				}
				json.NewDecoder(r.Body).Decode(&body)
				q := body.Statements[0]
				if len(q.Parameters.Docs) != 1 || q.Parameters.Docs[0] != docID || strings.Contains(q.Statement, "五行") || q.Parameters.Q != "五行" {
					t.Error("not scoped/parameterized")
				}
				w.Write([]byte(`{"results":[{"data":[{"row":[{"id":"node","name":"五行","document":"` + docID + `","chunks":["c"]},{"id":"other","name":"forbidden","document":"outside"},{"source":"node","target":"other","name":"links"}]}]}],"errors":[]}`))
			}))
			defer srv.Close()
			t.Setenv("AI_NEO4J_HTTP", srv.URL)
			t.Setenv("AI_NEO4J_PASSWORD", "private")
			db, m, _ := sqlmock.New()
			defer db.Close()
			s := &Service{DB: sqlx.NewSqlConnFromDB(db)}
			m.ExpectQuery("SELECT id,name,description,enabled,revision").WithArgs(kbID).WillReturnRows(sqlmock.NewRows(baseCols).AddRow(kbID, "name", "", true, 1))
			m.ExpectQuery("SELECT id,base_id,source,enabled,revision").WithArgs(kbID).WillReturnRows(sqlmock.NewRows(docCols).AddRow(docID, kbID, "出处", true, 1))
			m.ExpectQuery("SELECT id,base_id,source,enabled,revision").WithArgs(kbID).WillReturnRows(sqlmock.NewRows(docCols).AddRow(docID, kbID, "出处", !revoked, 1))
			m.ExpectQuery("SELECT id,name,description,enabled,revision").WithArgs(kbID).WillReturnRows(sqlmock.NewRows(baseCols).AddRow(kbID, "name", "", true, 1))
			out, e := s.EntityGraph(context.Background(), kbID, "五行")
			if e != nil {
				t.Fatal(e)
			}
			want := 1
			if revoked {
				want = 0
			}
			if len(out.Nodes) != want || len(out.Edges) != 0 {
				t.Fatalf("graph leaked: %+v", out)
			}
			if e = m.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}

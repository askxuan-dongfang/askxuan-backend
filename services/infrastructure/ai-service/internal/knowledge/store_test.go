package knowledge

import (
	"context"
	"encoding/json"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"regexp"
	"testing"
	"time"
)

type fakeEmbedding struct{}

func (fakeEmbedding) Identity() string                                 { return "test-zh" }
func (fakeEmbedding) Embed(context.Context, string) ([]float64, error) { return []float64{1, 0}, nil }
func TestSemanticRetrievalScopeExpiryAndOptOut(t *testing.T) {
	db, m, e := sqlmock.New()
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	s := Store{DB: sqlx.NewSqlConnFromDB(db), Embedder: fakeEmbedding{}}
	cols := []string{"id", "owner", "kind", "title", "body", "source", "locator", "enabled", "revision", "expires", "vector_json", "embedding_model"}
	rows := sqlmock.NewRows(cols).AddRow("mine", "17", "memory", "工作安排", "我偏好上午处理任务", "本人", "确认", true, 2, 0, "[1,0]", "test-zh").AddRow("disabled", "17", "memory", "偏好", "资料", "本人", "确认", false, 1, 0, "[1,0]", "test-zh").AddRow("expired", "17", "memory", "偏好", "资料", "本人", "确认", true, 1, time.Now().Unix()-1, "[1,0]", "test-zh")
	m.ExpectQuery(regexp.QuoteMeta("SELECT "+fields+" FROM ai_reference_entry WHERE kind=? AND owner=? ORDER BY id LIMIT 501")).WithArgs("memory", "17").WillReturnRows(rows)
	out, e := s.Search(context.Background(), "memory", "17", "早晨适合怎么规划")
	if e != nil {
		t.Fatal(e)
	}
	if out.Mode != "hybrid" || len(out.Hits) != 1 || out.Hits[0].ID != "mine" || len(out.Hits[0].Hash) != 64 {
		t.Fatalf("unexpected retrieval %+v", out)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
	if _, e = s.List(context.Background(), "memory", "platform"); e == nil {
		t.Fatal("invalid scope allowed")
	}
}
func TestMutationUsesOwnerAndRevision(t *testing.T) {
	db, m, _ := sqlmock.New()
	defer db.Close()
	s := Store{DB: sqlx.NewSqlConnFromDB(db)}
	e := Entry{ID: "owned-elsewhere", Owner: "7", Kind: "memory", Title: "偏好", Text: "简短回答", Revision: 1, Enabled: true}
	m.ExpectExec("UPDATE ai_reference_entry").WithArgs(e.Title, e.Text, "", "", true, int64(0), "[]", "", e.ID, "7", "memory", int64(1)).WillReturnResult(sqlmock.NewResult(0, 0))
	if _, err := s.Save(context.Background(), e); err != ErrNotFound {
		t.Fatal(err)
	}
	m.ExpectExec(`DELETE FROM ai_reference_entry WHERE id=\? AND kind=\? AND owner=\?`).WithArgs(e.ID, "memory", "7").WillReturnResult(sqlmock.NewResult(0, 0))
	if err := s.Delete(context.Background(), "memory", "7", e.ID); err != ErrNotFound {
		t.Fatal(err)
	}
	if e := m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
func TestChineseKeywordsAndPrivateFields(t *testing.T) {
	if lexical("想了解姓名笔画", "康熙姓名笔画") <= 0 {
		t.Fatal("Chinese retrieval missing")
	}
	if cosine([]float64{1}, []float64{1, 2}) != 0 {
		t.Fatal("mismatched vector used")
	}
	b, _ := json.Marshal(Entry{Owner: "secret", Vector: "private-vector"})
	if regexp.MustCompile("secret|private-vector").Match(b) {
		t.Fatal("private metadata exposed")
	}
}

func TestImportIsAtomicAndStartsDisabled(t *testing.T) {
	db, m, _ := sqlmock.New()
	defer db.Close()
	s := Store{DB: sqlx.NewSqlConnFromDB(db)}
	m.ExpectBegin()
	m.ExpectExec("INSERT IGNORE INTO ai_reference_scope").WithArgs("knowledge:platform").WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectQuery("SELECT scope_key").WithArgs("knowledge:platform").WillReturnRows(sqlmock.NewRows([]string{"scope_key"}).AddRow("knowledge:platform"))
	m.ExpectQuery("SELECT COUNT").WithArgs("knowledge", "platform").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	m.ExpectExec("INSERT INTO ai_reference_entry").WithArgs(sqlmock.AnyArg(), "platform", "knowledge", "测试资料", "第一行\n第二行", "测试授权文件", "行 1–2", false, int64(1), int64(0), "[]", "").WillReturnResult(sqlmock.NewResult(1, 1))
	m.ExpectCommit()
	n, e := s.ImportText(context.Background(), "测试资料", "测试授权文件", "第一行\n第二行")
	if e != nil || n != 1 {
		t.Fatalf("%d %v", n, e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
	if _, e = s.ImportText(context.Background(), "标题", "", "内容"); e == nil {
		t.Fatal("source missing allowed")
	}
}
func TestDisableDoesNotRequireEmbedding(t *testing.T) {
	db, m, _ := sqlmock.New()
	defer db.Close()
	s := Store{DB: sqlx.NewSqlConnFromDB(db), Embedder: unavailableEmbedding{}}
	m.ExpectExec("UPDATE ai_reference_entry").WillReturnResult(sqlmock.NewResult(0, 1))
	_, e := s.Save(context.Background(), Entry{ID: "mine", Owner: "7", Kind: "memory", Title: "过期记忆", Text: "偏好", Revision: 1, Expires: 1, Enabled: false})
	if e != nil {
		t.Fatal(e)
	}
	if e = m.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}

type unavailableEmbedding struct{}

func (unavailableEmbedding) Identity() string                                 { return "offline" }
func (unavailableEmbedding) Embed(context.Context, string) ([]float64, error) { return nil, ErrInput }

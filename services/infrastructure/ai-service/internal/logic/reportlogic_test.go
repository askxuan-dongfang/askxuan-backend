package logic

import (
	"context"
	"encoding/json"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"strings"
	"testing"
)

func TestReportPaidContentBoundary(t *testing.T) {
	for _, tc := range []struct {
		name         string
		points, cash int64
		want         bool
	}{{"unpaid", 0, 0, false}, {"points", 1, 0, true}, {"cash", 0, 1, true}, {"refunded", 0, 0, false}} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, e := sqlmock.New()
			if e != nil {
				t.Fatal(e)
			}
			defer db.Close()
			row := sqlmock.NewRows(strings.Split(reportCols, ",")).AddRow(1, "AR1", "100", "bazi", "八字", "1", "问题", "{}", "[\"资料\"]", 990, 10, "ready", "摘要", "SECRET CONTENT", "", tc.points, "2026-09-09")
			mock.ExpectQuery("SELECT .* FROM ai_report WHERE id=\\? AND user_id=\\?").WithArgs(int64(1), "100").WillReturnRows(row)
			mock.ExpectQuery("SELECT COUNT.*askxuan_payment.payment").WithArgs("AR1", "100", int64(990)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(tc.cash))
			r, e := ReportGet(context.Background(), &svc.ServiceContext{DB: sqlx.NewSqlConnFromDB(db)}, "100", 1)
			if e != nil {
				t.Fatal(e)
			}
			data, _ := json.Marshal(r)
			if strings.Contains(string(data), "SECRET CONTENT") != tc.want || r.Unlocked != tc.want {
				t.Fatalf("unexpected entitlement: %s", data)
			}
			if e := mock.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestReportOtherUserCannotRead(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	mock.ExpectQuery("SELECT .* FROM ai_report").WithArgs(int64(1), "attacker").WillReturnRows(sqlmock.NewRows(strings.Split(reportCols, ",")))
	r, e := ReportGet(context.Background(), &svc.ServiceContext{DB: sqlx.NewSqlConnFromDB(db)}, "attacker", 1)
	if e == nil || r != nil {
		t.Fatal("cross-user disclosure")
	}
	if e := mock.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}
func TestReportRejectsShortOrUnstructuredDelivery(t *testing.T) {
	for _, b := range []reportBody{{"摘要", "短文"}, {strings.Repeat("摘", 30), strings.Repeat("长", 350)}, {"", strings.Repeat("## 章节\n内容", 100)}} {
		if validReportBody(b) {
			t.Fatal("accepted incomplete report")
		}
	}
	if !validReportBody(reportBody{strings.Repeat("摘", 30), strings.Repeat("## 章节\n内容", 60)}) {
		t.Fatal("rejected structured report")
	}
}

func TestReportCatalogIncludesChapters(t *testing.T) {
	db, m, _ := sqlmock.New()
	defer db.Close()
	m.ExpectQuery("SELECT p.code").WillReturnRows(sqlmock.NewRows([]string{"code", "title", "subtitle", "price_cents", "points_price", "chapters_json", "version"}).AddRow("bazi", "八字", "介绍", 990, 10, `["资料","分析"]`, "1"))
	rows, e := ReportProducts(context.Background(), &svc.ServiceContext{DB: sqlx.NewSqlConnFromDB(db)})
	if e != nil || len(rows) != 1 || len(rows[0].Chapters) != 2 {
		t.Fatalf("catalog %v %v", rows, e)
	}
}

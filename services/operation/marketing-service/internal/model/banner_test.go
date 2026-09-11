package model

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/askxuan/marketing-service/internal/types"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"testing"
)

func bannerFixtureRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "title", "placement", "image_url", "link_type", "link_value", "sort", "status", "start_time", "end_time", "created_at"}).AddRow(7, "fixture", "customer_home", "/assets/image.jpg", "ai", "", 9, "disabled", "", "", "2026-09-11 10:00:00")
}
func TestBannerPatchPreservesOmittedFieldsAndClearsDates(t *testing.T) {
	for _, body := range []string{`{"status":"disabled"}`, `{"sort":0,"startTime":"","endTime":""}`} {
		t.Run(body, func(t *testing.T) {
			conn, m, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			old := db
			db = sqlx.NewSqlConnFromDB(conn)
			defer func() { db = old }()
			var p types.BannerUpdateReq
			if err = json.Unmarshal([]byte(body), &p); err != nil {
				t.Fatal(err)
			}
			q := m.ExpectExec(`UPDATE banner SET .*sort=COALESCE\(\?,sort\).*start_time=CASE WHEN \? IS NULL THEN start_time ELSE NULLIF\(\?,''\) END`)
			if p.Status != nil {
				q.WithArgs(nil, nil, nil, nil, nil, nil, "disabled", nil, nil, nil, nil, int64(7))
			} else {
				q.WithArgs(nil, nil, nil, nil, nil, 0, nil, "", "", "", "", int64(7))
			}
			q.WillReturnResult(sqlmock.NewResult(0, 0))
			m.ExpectQuery(`SELECT .* FROM banner WHERE id=\?`).WithArgs(int64(7)).WillReturnRows(bannerFixtureRows())
			if _, err = UpdateBanner(context.Background(), 7, p); err != nil {
				t.Fatal(err)
			}
			if err = m.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestActiveBannerPaginationUsesFilteredCount(t *testing.T) {
	conn, m, _ := sqlmock.New()
	defer conn.Close()
	old := db
	db = sqlx.NewSqlConnFromDB(conn)
	defer func() { db = old }()
	when := "2026-09-11 14:00:00"
	filter := `WHERE 1=1 AND status=\? AND placement=\? AND \(start_time IS NULL OR start_time<=\?\) AND \(end_time IS NULL OR end_time>=\?\)`
	m.ExpectQuery(`SELECT COUNT\(1\) FROM banner `+filter).WithArgs("enabled", "customer_home", when, when).WillReturnRows(sqlmock.NewRows([]string{"total"}).AddRow(21))
	m.ExpectQuery(`SELECT .* FROM banner `+filter+` ORDER BY sort,id DESC LIMIT \?,\?`).WithArgs("enabled", "customer_home", when, when, 20, 20).WillReturnRows(bannerFixtureRows())
	list, total, err := ListBanners(context.Background(), "enabled", "customer_home", 2, 20, when)
	if err != nil || total != 21 || len(list) != 1 {
		t.Fatalf("list=%v total=%d err=%v", list, total, err)
	}
	if err = m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestBannerInsertFailureIsNotSuccessWithZeroID(t *testing.T) {
	conn, m, _ := sqlmock.New()
	defer conn.Close()
	old := db
	db = sqlx.NewSqlConnFromDB(conn)
	defer func() { db = old }()
	m.ExpectExec(`INSERT INTO banner`).WillReturnError(errors.New("storage unavailable"))
	if _, err := InsertBanner(context.Background(), Banner{Title: "fixture", Placement: BannerPlacementHome}); err == nil {
		t.Fatal("storage failure was swallowed")
	}
}

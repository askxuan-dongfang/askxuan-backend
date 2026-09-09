package model

import (
	"encoding/json"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/askxuan/marketing-service/internal/types"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestCouponStatusPatchPreservesOmittedValues(t *testing.T) {
	for _, body := range []string{`{"status":"disabled"}`, `{"minAmount":0,"totalCount":0,"categoryId":""}`} {
		t.Run(body, func(t *testing.T) {
			var req types.CouponUpdateReq
			if err := json.Unmarshal([]byte(body), &req); err != nil {
				t.Fatal(err)
			}
			conn, m, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			previous := db
			db = sqlx.NewSqlConnFromDB(conn)
			defer func() { db = previous }()
			query := m.ExpectExec(`UPDATE coupon SET .*value=COALESCE\(\?,value\),min_amount=COALESCE\(\?,min_amount\),category_id=COALESCE\(\?,category_id\).*total_count=COALESCE\(\?,total_count\)`)
			if req.Status != "" {
				query.WithArgs("", "", nil, nil, nil, "", "", nil, "disabled", int64(9))
			} else {
				query.WithArgs("", "", nil, float64(0), "", "", "", int64(0), "", int64(9))
			}
			query.WillReturnResult(sqlmock.NewResult(0, 1))
			m.ExpectQuery(`SELECT .* FROM coupon WHERE id=\?`).WithArgs(int64(9)).WillReturnRows(sqlmock.NewRows([]string{"id", "coupon_no", "name", "type", "value", "min_amount", "category_id", "start_time", "end_time", "total_count", "received_count", "status", "created_at"}).AddRow(9, "TEST9", "fixture", "full_reduce", 10, 100, "category", "2026-01-01 00:00:00", "2026-12-31 23:59:59", 100, 1, "disabled", "2026-01-01 00:00:00"))
			_, ok := UpdateCoupon(9, CouponPatch{Value: req.Value, MinAmount: req.MinAmount, CategoryId: req.CategoryId, TotalCount: req.TotalCount, Status: req.Status})
			if !ok {
				t.Fatal("update failed")
			}
			if err := m.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

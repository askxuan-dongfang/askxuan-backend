package server

import (
	"context"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/askxuan/common/rpc/catalog"
	"github.com/askxuan/product-service/internal/svc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
)

func productRows(experience bool) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "product_no", "name", "category_id", "description", "main_image", "status", "price", "market_price", "stock", "tags", "freight_template_id", "is_experience", "source_name", "source_url", "source_note", "create_time", "update_time"}).AddRow(1, "EXP-case", "案例", 1, "", "/catalog-experiences/case.jpg", "on_shelf", 59, 0, 20, "", 0, experience, "来源", "https://example.com", "", "2026-09-11", "2026-09-11")
}
func TestReservationExperienceSnapshotAndAggregateStock(t *testing.T) {
	db, m, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := NewCatalogServer(&svc.ServiceContext{DB: sqlx.NewSqlConnFromDB(db)})
	m.ExpectQuery("SELECT status,snapshot FROM product_stock_reservation").WillReturnRows(sqlmock.NewRows([]string{"status", "snapshot"}))
	m.ExpectBegin()
	m.ExpectQuery("SELECT id,product_no").WithArgs(int64(1)).WillReturnRows(productRows(true))
	m.ExpectQuery("SELECT id,product_id").WithArgs(int64(11), int64(1)).WillReturnRows(sqlmock.NewRows([]string{"id", "product_id", "spec_name", "spec_value", "price", "stock", "sku_no"}).AddRow(11, 1, "款式", "檀木", 59, 20, "EXP-11"))
	m.ExpectExec("UPDATE product_sku SET stock=stock-\\?").WithArgs(int32(2), int64(11), int32(2)).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec("UPDATE product SET stock=").WithArgs(int64(1), int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	m.ExpectExec("INSERT INTO product_stock_reservation").WithArgs("case-1", sqlmock.AnyArg()).WillReturnResult(sqlmock.NewResult(1, 1))
	m.ExpectCommit()
	got, err := s.ReserveCart(context.Background(), &catalog.ReserveCartReq{RequestId: "case-1", Items: []*catalog.CartLine{{ProductId: 1, SkuId: 11, Quantity: 2}}})
	if err != nil {
		t.Fatal(err)
	}
	if !got.Items[0].IsExperience || got.TotalAmount != 118 {
		t.Fatal(got)
	}
	if err = m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
func TestNoSkuBypassForConfiguredVariants(t *testing.T) {
	db, m, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	s := NewCatalogServer(&svc.ServiceContext{DB: sqlx.NewSqlConnFromDB(db)})
	m.ExpectQuery("SELECT status,snapshot").WillReturnRows(sqlmock.NewRows([]string{"status", "snapshot"}))
	m.ExpectBegin()
	m.ExpectQuery("SELECT id,product_no").WillReturnRows(productRows(true))
	m.ExpectQuery("SELECT COUNT").WithArgs(int64(1)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
	m.ExpectRollback()
	m.ExpectQuery("SELECT status,snapshot").WillReturnRows(sqlmock.NewRows([]string{"status", "snapshot"}))
	_, err = s.ReserveCart(context.Background(), &catalog.ReserveCartReq{RequestId: "case-2", Items: []*catalog.CartLine{{ProductId: 1, Quantity: 1}}})
	if status.Code(err) != codes.InvalidArgument {
		t.Fatal(err)
	}
	if err = m.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

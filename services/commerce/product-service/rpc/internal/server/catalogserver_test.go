package server

import (
	"context"
	"os"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/askxuan/common/rpc/catalog"
	"github.com/askxuan/product-service/internal/svc"
	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"testing"
)

func productRows(experience bool) *sqlmock.Rows {
	return sqlmock.NewRows([]string{"id", "product_no", "name", "category_id", "description", "main_image", "status", "price", "market_price", "stock", "tags", "freight_template_id", "is_experience", "source_name", "source_url", "source_note", "create_time", "update_time"}).AddRow(1, "EXP-case", "案例", 1, "", "/catalog-experiences/case.jpg", "on_shelf", 59, 0, 20, "", 51, experience, "来源", "https://example.com", "", "2026-09-11", "2026-09-11")
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
	if !got.Items[0].IsExperience || got.Items[0].FreightTemplateId != 51 || got.TotalAmount != 118 {
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

func TestExperienceCatalogStockMySQL(t *testing.T) {
	dsn := os.Getenv("WISH_CATALOG_TEST_DSN")
	if dsn == "" || os.Getenv("WISH_CHECKOUT_TEST_ISOLATED") != "1" {
		t.Skip("requires an isolated database loaded with the experience fixtures")
	}
	ctx := context.Background()
	db := sqlx.NewMysql(dsn)
	server := NewCatalogServer(&svc.ServiceContext{DB: db})
	var product struct {
		ID    int64 `db:"id"`
		Stock int   `db:"stock"`
	}
	if err := db.QueryRowCtx(ctx, &product, `SELECT id,stock FROM product WHERE product_no='EXP-1120-BRACELET'`); err != nil {
		t.Fatal(err)
	}
	key := uuid.NewString()
	req := &catalog.ReserveCartReq{RequestId: key, Items: []*catalog.CartLine{{ProductId: product.ID, Quantity: 3}}}
	release := &catalog.ReleaseCartReq{RequestId: key}
	t.Cleanup(func() { _, _ = server.ReleaseCart(ctx, release) })
	reserved, err := server.ReserveCart(ctx, req)
	if err != nil || reserved.Items[0].FreightTemplateId != 991120 || reserved.TotalAmount != 147 {
		t.Fatalf("reserve: %+v, %v", reserved, err)
	}
	assertStock := func(want int) {
		t.Helper()
		var stock int
		if err := db.QueryRowCtx(ctx, &stock, `SELECT stock FROM product WHERE id=?`, product.ID); err != nil || stock != want {
			t.Fatalf("stock=%d want=%d err=%v", stock, want, err)
		}
	}
	assertStock(product.Stock - 3)
	for _, want := range []bool{true, false} {
		result, err := server.ReleaseCart(ctx, release)
		if err != nil || result.Released != want {
			t.Fatalf("release: %+v err=%v", result, err)
		}
		assertStock(product.Stock)
	}
	if _, err := server.ReserveCart(ctx, req); status.Code(err) != codes.FailedPrecondition {
		t.Fatalf("released request reused: %v", err)
	}
	assertStock(product.Stock)
}

package model

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestDiyOrderRowsNormalizeLegacyNullableFields(t *testing.T) {
	for _, column := range []string{"source", "creator_id", "design_snapshot", "pricing_snapshot"} {
		if !strings.Contains(diyOrderRows, "COALESCE("+column+",'')") {
			t.Fatalf("legacy nullable column %s is not normalized", column)
		}
	}
}

func TestCreatePricedOrderIgnoresSnapshotPrice(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	conn := sqlx.NewSqlConnFromDB(db)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id,name,spec,unit_price.*FROM material").WithArgs(int64(1)).WillReturnRows(
		sqlmock.NewRows([]string{"id", "name", "spec", "unit_price", "unit", "category", "five_elements", "image", "stock", "status"}).
			AddRow(1, "紫檀", "10mm", 28.0, "颗", "main_bead", "wood", "", 10, MaterialStatusOnShelf),
	)
	mock.ExpectQuery("SELECT id,material_id,spec,price,stock FROM material_sku").WithArgs(int64(1), "10mm").WillReturnRows(
		sqlmock.NewRows([]string{"id", "material_id", "spec", "price", "stock"}).AddRow(7, 1, "10mm", 30.0, 8),
	)
	mock.ExpectExec("UPDATE material SET stock=stock-").WithArgs(2, int64(1), MaterialStatusOnShelf, 2).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE material_sku SET stock=stock-").WithArgs(2, int64(7), 2).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT config_value FROM diy_config").WillReturnRows(sqlmock.NewRows([]string{"config_value"}).AddRow("0"))
	mock.ExpectExec("INSERT INTO askxuan_diy.diy_order").WillReturnResult(sqlmock.NewResult(11, 1))
	mock.ExpectExec("INSERT INTO askxuan_diy.diy_order_item").WillReturnResult(sqlmock.NewResult(21, 1))
	mock.ExpectCommit()

	result, err := CreatePricedOrder(context.Background(), conn, NewDiyOrderModel(conn), NewDiyOrderItemModel(conn), PricedOrderInput{
		UserId: "buyer", Design: &DiyDesign{Id: 9, UserId: "creator", TotalPrice: 2},
		Items:     []PricedOrderItemInput{{MaterialId: 1, Spec: "10mm", Quantity: 2, SnapshotUnitPrice: 1}},
		AddressId: 1, Source: "design_square", CreatorId: "creator",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Order.MaterialFee != 60 || result.Order.OriginalMaterialFee != 2 || result.Order.PriceChanged != 1 {
		t.Fatalf("unexpected pricing result: %#v", result.Order)
	}
	if result.Items[0].UnitPrice != 30 || result.Items[0].SkuId != 7 {
		t.Fatalf("snapshot price was trusted: %#v", result.Items[0])
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCreatePricedOrderRollsBackWhenItemInsertFails(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	conn := sqlx.NewSqlConnFromDB(db)

	mock.ExpectBegin()
	mock.ExpectQuery("SELECT id,name,spec,unit_price.*FROM material").WillReturnRows(
		sqlmock.NewRows([]string{"id", "name", "spec", "unit_price", "unit", "category", "five_elements", "image", "stock", "status"}).
			AddRow(1, "紫檀", "10mm", 28.0, "颗", "main_bead", "wood", "", 10, MaterialStatusOnShelf),
	)
	mock.ExpectQuery("SELECT id,material_id,spec,price,stock FROM material_sku").WillReturnRows(sqlmock.NewRows([]string{"id", "material_id", "spec", "price", "stock"}))
	mock.ExpectExec("UPDATE material SET stock=stock-").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT INTO askxuan_diy.diy_order").WillReturnResult(sqlmock.NewResult(11, 1))
	mock.ExpectExec("INSERT INTO askxuan_diy.diy_order_item").WillReturnError(errors.New("forced item failure"))
	mock.ExpectRollback()

	_, err = CreatePricedOrder(context.Background(), conn, NewDiyOrderModel(conn), NewDiyOrderItemModel(conn), PricedOrderInput{
		UserId: "buyer", Design: &DiyDesign{Id: 9},
		Items:     []PricedOrderItemInput{{MaterialId: 1, Spec: "10mm", Quantity: 1, SnapshotUnitPrice: 28}},
		AddressId: 1, Source: "custom",
	})
	if err == nil {
		t.Fatal("expected item insert failure")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCreatorEarningCreatedOnlyAfterPaymentSuccess(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	conn := sqlx.NewSqlConnFromDB(db)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT " + diyOrderRows + " FROM diy_order")).WithArgs("DIY001").WillReturnRows(
		sqlmock.NewRows([]string{"id", "order_no", "user_id", "design_id", "material_fee", "bless_fee", "total_fee", "status", "payment_status", "address_id", "source", "creator_id", "creator_share_rate", "original_material_fee", "price_changed", "design_snapshot", "pricing_snapshot", "create_time", "update_time"}).
			AddRow(12, "DIY001", "buyer", 9, 100.0, 0.0, 100.0, DiyStatusPendingReview, "pending", 1, "design_square", "creator", 0.1, 100.0, 0, "{}", "{}", "2026-07-13", "2026-07-13"),
	)
	mock.ExpectExec("UPDATE diy_order SET payment_status=").WithArgs(int64(12)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("INSERT IGNORE INTO diy_creator_earning").WithArgs(sqlmock.AnyArg(), int64(12), "DIY001", int64(9), "creator", "PAY001", 100.0, 0.1, 10.0, CreatorEarningStatusPending).WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	if err := NewCreatorEarningModel(conn).RecordPaymentSuccess(context.Background(), "DIY001", "PAY001"); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCancelAndRestockUsesSingleTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	conn := sqlx.NewSqlConnFromDB(db)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT " + diyOrderRows + " FROM askxuan_diy.diy_order")).WithArgs(int64(12)).WillReturnRows(
		sqlmock.NewRows([]string{"id", "order_no", "user_id", "design_id", "material_fee", "bless_fee", "total_fee", "status", "payment_status", "address_id", "source", "creator_id", "creator_share_rate", "original_material_fee", "price_changed", "design_snapshot", "pricing_snapshot", "create_time", "update_time"}).
			AddRow(12, "DIY001", "buyer", 9, 60.0, 0.0, 60.0, DiyStatusPendingReview, "pending", 1, "design_square", "creator", 0.0, 2.0, 1, "{}", "{}", "2026-07-13", "2026-07-13"),
	)
	mock.ExpectQuery("SELECT id,order_id,material_id,sku_id.*FROM askxuan_diy.diy_order_item").WithArgs(int64(12)).WillReturnRows(
		sqlmock.NewRows([]string{"id", "order_id", "material_id", "sku_id", "material_name", "spec", "unit_price", "quantity", "subtype"}).
			AddRow(21, 12, 1, 7, "紫檀", "10mm", 30.0, 2, "main_bead"),
	)
	mock.ExpectExec("UPDATE askxuan_diy.material SET stock=stock\\+?").WithArgs(2, int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE askxuan_diy.material_sku SET stock=stock\\+?").WithArgs(2, int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE askxuan_diy.diy_order SET status=").WithArgs(DiyStatusCancelled, sqlmock.AnyArg(), int64(12), DiyStatusPendingReview).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	order, err := NewDiyOrderModel(conn).CancelAndRestock(context.Background(), 12)
	if err != nil {
		t.Fatal(err)
	}
	if order.Status != DiyStatusCancelled {
		t.Fatalf("unexpected status: %s", order.Status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCancelAndRestockRollsBackOnInventoryFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	conn := sqlx.NewSqlConnFromDB(db)

	mock.ExpectBegin()
	mock.ExpectQuery(regexp.QuoteMeta("SELECT " + diyOrderRows + " FROM askxuan_diy.diy_order")).WillReturnRows(
		sqlmock.NewRows([]string{"id", "order_no", "user_id", "design_id", "material_fee", "bless_fee", "total_fee", "status", "payment_status", "address_id", "source", "creator_id", "creator_share_rate", "original_material_fee", "price_changed", "design_snapshot", "pricing_snapshot", "create_time", "update_time"}).
			AddRow(12, "DIY001", "buyer", 9, 60.0, 0.0, 60.0, DiyStatusPendingReview, "pending", 1, "design_square", "creator", 0.0, 2.0, 1, "{}", "{}", "2026-07-13", "2026-07-13"),
	)
	mock.ExpectQuery("SELECT id,order_id,material_id,sku_id.*FROM askxuan_diy.diy_order_item").WillReturnRows(
		sqlmock.NewRows([]string{"id", "order_id", "material_id", "sku_id", "material_name", "spec", "unit_price", "quantity", "subtype"}).
			AddRow(21, 12, 1, 7, "紫檀", "10mm", 30.0, 2, "main_bead"),
	)
	mock.ExpectExec("UPDATE askxuan_diy.material SET stock=stock\\+?").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE askxuan_diy.material_sku SET stock=stock\\+?").WillReturnError(errors.New("forced restock failure"))
	mock.ExpectRollback()

	if _, err := NewDiyOrderModel(conn).CancelAndRestock(context.Background(), 12); err == nil {
		t.Fatal("expected restock failure")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

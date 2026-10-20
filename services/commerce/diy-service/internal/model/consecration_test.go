package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestConsecrationCodeValidation(t *testing.T) {
	for _, code := range []string{"E001", "consecration:temple:0", "consecration:master:-2", "consecration:temple:01", "consecration:other:1"} {
		if ValidConsecrationCode(code) {
			t.Fatalf("accepted %q", code)
		}
	}
	for _, code := range []string{"consecration:temple:1", "consecration:master:123"} {
		if !ValidConsecrationCode(code) {
			t.Fatalf("rejected %q", code)
		}
	}
}

func TestConsecrationPricingAndSnapshot(t *testing.T) {
	for _, tc := range []struct {
		name, code     string
		live, expected float64
		want           error
	}{
		{"temple original price", "consecration:temple:7", 39.50, 39.50, nil},
		{"master zero fee", "consecration:master:7", 0, 0, nil},
		{"price changed", "consecration:temple:7", 49, 39.50, ErrConsecrationPriceChanged},
		{"legacy markup rejected", "E001", 168, 168, ErrOrderBlessingUnavailable},
		{"no service does not inherit saved design", "", 0, 0, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, e := sqlmock.New()
			if e != nil {
				t.Fatal(e)
			}
			defer db.Close()
			conn := sqlx.NewSqlConnFromDB(db)
			ctx := context.Background()
			mock.ExpectBegin()
			expectOrderDraft(mock, 1)
			mock.ExpectQuery("SELECT id,name,spec,unit_price.*FROM material").WillReturnRows(sqlmock.NewRows([]string{"id", "name", "spec", "unit_price", "unit", "category", "five_elements", "image", "stock", "status"}).AddRow(1, "珠子", "10mm", 28, "颗", "main_bead", "wood", "", 10, "on_shelf"))
			mock.ExpectQuery("SELECT id,material_id,spec,price,stock FROM material_sku").WillReturnRows(sqlmock.NewRows([]string{"id", "material_id", "spec", "price", "stock"}))
			mock.ExpectExec("UPDATE material SET stock=stock-").WillReturnResult(sqlmock.NewResult(0, 1))
			if ValidConsecrationCode(tc.code) {
				provider, temple, master := "temple", "T001", ""
				if strings.Contains(tc.code, ":master:") {
					provider, temple, master = "master", "", "M001"
				}
				mock.ExpectQuery("SELECT \\* FROM").WithArgs(tc.code).WillReturnRows(sqlmock.NewRows([]string{"id", "service_code", "service_name", "source_service_id", "source_service_code", "provider_type", "temple_code", "temple_name", "master_code", "master_name", "price"}).AddRow(7, tc.code, "开光", 7, "S006", provider, temple, "寺院", master, "法师", tc.live))
			}
			if tc.want != nil {
				mock.ExpectRollback()
			} else {
				mock.ExpectExec("INSERT INTO askxuan_diy.diy_order").WillReturnResult(sqlmock.NewResult(11, 1))
				mock.ExpectExec("INSERT INTO askxuan_diy.diy_order_item").WillReturnResult(sqlmock.NewResult(21, 1))
				mock.ExpectCommit()
			}
			design := testOrderDraft(1)
			result, err := CreatePricedOrder(ctx, conn, NewDiyOrderModel(conn), NewDiyOrderItemModel(conn), PricedOrderInput{UserId: "buyer", Design: design, Items: []PricedOrderItemInput{{MaterialId: 1, Spec: "10mm", Quantity: 1, SnapshotUnitPrice: 28}}, BlessServiceCode: tc.code, ExpectedBlessFee: &tc.expected, AddressId: 1, Source: "custom"})
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v want %v", err, tc.want)
			}
			if err == nil {
				if result.Order.TotalFee != 28+tc.live || result.Order.BlessFee != tc.live {
					t.Fatalf("unexpected charges %+v", result.Order)
				}
				var saved DiyDesign
				if json.Unmarshal([]byte(result.Order.DesignSnapshot), &saved) != nil || saved.BlessServiceCode != tc.code {
					t.Fatal("selected service missing from immutable design snapshot")
				}
				if design.BlessServiceCode != "E001" {
					t.Fatal("modified source design")
				}
				if tc.code != "" {
					svc, e := orderConsecration(result.Order)
					if e != nil || svc.ServiceCode != tc.code || svc.Price != tc.live {
						t.Fatal("provider snapshot missing", e)
					}
				}
			}
			if e := mock.ExpectationsWereMet(); e != nil {
				t.Fatal(e)
			}
		})
	}
}

// Run only against the dedicated disposable container initialized from db/init.sql.
func TestConsecrationMySQL(t *testing.T) {
	dsn := os.Getenv("CONSECRATION_TEST_DSN")
	if dsn == "" {
		t.Skip("dedicated isolated MySQL required")
	}
	if !strings.Contains(dsn, "127.0.0.1:33418") || !strings.Contains(dsn, "/askxuan_diy") {
		t.Fatal("only dedicated local database permitted")
	}
	db := sqlx.NewMysql(dsn)
	ctx := context.Background()
	var host string
	if e := db.QueryRowCtx(ctx, &host, "SELECT @@hostname"); e != nil || host != "askxuan-consecration-test" {
		t.Fatal("wrong test container", e)
	}
	exec := func(q string, args ...interface{}) {
		t.Helper()
		if _, e := db.ExecCtx(ctx, q, args...); e != nil {
			t.Fatal(e)
		}
	}
	exec("UPDATE askxuan_temple.temple SET status='正常' WHERE code='T001'")
	exec("INSERT INTO askxuan_temple.temple_service(temple_code,service_code,service_name,price,status) VALUES('T001','S006','开光',39.50,'on_shelf') ON DUPLICATE KEY UPDATE price=39.50,status='on_shelf'")
	exec("UPDATE askxuan_master.master SET manage_by='platform',auth_status='已认证',shelf_status='on_shelf',platform_status='normal' WHERE code='M001'")
	exec("INSERT INTO askxuan_master.master_service_tag(master_code,service_code,price,status) VALUES('M001','S006',0,'enabled') ON DUPLICATE KEY UPDATE price=0,status='enabled'")
	rows, _, e := ListConsecrations(ctx, db, 1, 100)
	if e != nil {
		t.Fatal(e)
	}
	selected := map[string]Consecration{}
	for _, v := range rows {
		if v.TempleCode == "T001" || v.MasterCode == "M001" {
			selected[v.ProviderType] = v
		}
	}
	if len(selected) != 2 || selected["temple"].Price != 39.50 || selected["master"].Price != 0 {
		t.Fatalf("wrong catalog %+v", selected)
	}
	design, e := NewDiyDesignModel(db).Insert(ctx, &DiyDesign{UserId: "consecration-test", Name: "consecration fixture",
		DesignData: testOrderDraft(1).DesignData, TotalPrice: 28, Status: DesignStatusPrivate})
	if e != nil {
		t.Fatal(e)
	}
	for _, provider := range []string{"temple", "master"} {
		service := selected[provider]
		in := PricedOrderInput{UserId: "consecration-test", Design: design, Items: []PricedOrderItemInput{{MaterialId: 1, Spec: "10mm", Quantity: 1, SnapshotUnitPrice: 28}}, BlessServiceCode: service.ServiceCode, ExpectedBlessFee: &service.Price, AddressId: 1, Source: "custom"}
		result, e := CreatePricedOrder(ctx, db, NewDiyOrderModel(db), NewDiyOrderItemModel(db), in)
		if e != nil {
			t.Fatal(e)
		}
		if result.Order.TotalFee != result.Order.MaterialFee+service.Price {
			t.Fatal("additional charge")
		}
		// Change source catalog after checkout; paid order must retain its provider/fee.
		if provider == "temple" {
			exec("UPDATE askxuan_temple.temple_service SET price=199,status='off_shelf' WHERE id=?", service.SourceServiceId)
		} else {
			exec("UPDATE askxuan_master.master_service_tag SET price=199,status='disabled' WHERE id=?", service.SourceServiceId)
		}
		exec("UPDATE diy_order SET status='in_making',payment_status='success' WHERE id=?", result.Order.Id)
		updated, task, e := NewDiyOrderModel(db).CompleteMaking(ctx, result.Order.Id, service.ServiceCode)
		if e != nil || updated.Status != DiyStatusAwaitingBlessing || task.TempleCode != service.TempleCode || task.MasterCode != service.MasterCode {
			t.Fatalf("dispatch failed %+v %v", task, e)
		}
		if _, _, e = NewDiyOrderModel(db).CompleteMaking(ctx, result.Order.Id, service.ServiceCode); !errors.Is(e, ErrDiyOrderStateConflict) {
			t.Fatal("duplicate dispatch allowed", e)
		}
		if _, e = CreatePricedOrder(ctx, db, NewDiyOrderModel(db), NewDiyOrderItemModel(db), in); !errors.Is(e, ErrOrderBlessingUnavailable) {
			t.Fatal("unavailable service accepted", e)
		}
		t.Logf("%s: original fee %.2f preserved; task %s assigned; duplicate rejected", provider, updated.BlessFee, task.TaskNo)
	}
	// Only on-shelf independent masters can be selected; non-S006 offerings never appear.
	exec("UPDATE askxuan_master.master_service_tag SET status='enabled' WHERE master_code='M001'")
	exec("UPDATE askxuan_master.master SET manage_by='temple' WHERE code='M001'")
	rows, _, e = ListConsecrations(ctx, db, 1, 100)
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range rows {
		if v.MasterCode == "M001" || v.SourceServiceCode != "S006" {
			t.Fatal("invalid offering", fmt.Sprint(v))
		}
	}
}

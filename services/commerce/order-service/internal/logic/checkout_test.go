package logic

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"

	"github.com/askxuan/common"
	"github.com/askxuan/common/rpc/catalog"
	"github.com/askxuan/order-service/internal/model"
	"github.com/askxuan/order-service/internal/svc"
	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestPieceFreight(t *testing.T) {
	config := `{"default":{"first":1,"firstFee":12.45,"next":2,"nextFee":3.05},"regions":{"上海市":{"first":2,"firstFee":8,"next":1,"nextFee":2}}}`
	for _, tc := range []struct {
		province string
		pieces   int64
		want     int64
	}{
		{"上海市", 2, 800},
		{"上海", 3, 1000},
		{"浙江省", 1, 1245},
		{"浙江省", 2, 1550},
		{"浙江省", 4, 1855},
	} {
		if got, ok := pieceFreight(config, tc.province, tc.pieces); !ok || got != tc.want {
			t.Errorf("%s %d 件运费 = %d, %t, want %d", tc.province, tc.pieces, got, ok, tc.want)
		}
	}
	if _, ok := pieceFreight(`{"first":1,"next":1,"nextFee":0}`, "上海市", 1); ok {
		t.Fatal("missing base fee accepted")
	}
}

func TestExperienceShopCheckoutMySQL(t *testing.T) {
	dsn := os.Getenv("WISH_CHECKOUT_TEST_DSN")
	if dsn == "" || os.Getenv("WISH_CHECKOUT_TEST_ISOLATED") != "1" {
		t.Skip("requires an isolated database loaded with the experience fixtures")
	}
	db := sqlx.NewMysql(dsn)
	ctx := context.Background()
	items := []*catalog.CatalogQuote{
		{ProductId: 991120, Quantity: 1, IsExperience: true},
		{ProductId: 991121, Quantity: 2, IsExperience: true},
	}
	for id, want := range map[int64]int64{991120: 1000, 991121: 1800} {
		got, err := calculateFreight(ctx, db, 991120, id, items)
		if err != nil || got != want {
			t.Fatalf("address %d: freight=%d want=%d err=%v", id, got, want, err)
		}
	}
	if _, err := calculateFreight(ctx, db, 991121, 991120, items); err == nil {
		t.Fatal("another user's address was accepted")
	}
	orders := model.NewShopOrderModel(db)
	create := func() *model.ShopOrder {
		o, err := orders.Insert(ctx, &model.ShopOrder{
			RequestId: uuid.NewString(), UserId: "991120", TotalAmount: 177, PayAmount: 177, FreightAmount: 10,
		})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = db.ExecCtx(ctx, "DELETE FROM shop_order WHERE id=?", o.Id) })
		return o
	}
	cancelled, expired, paid := create(), create(), create()
	if err := orders.ClosePending(ctx, cancelled.Id, "991121"); !errors.Is(err, common.ErrForbidden) {
		t.Fatalf("foreign cancellation: %v", err)
	}
	if err := orders.ClosePending(ctx, cancelled.Id, "991120"); err != nil {
		t.Fatal(err)
	}
	if err := orders.ClosePending(ctx, cancelled.Id, "991120"); err != nil {
		t.Fatalf("repeated cancellation: %v", err)
	}
	if _, err := db.ExecCtx(ctx, "UPDATE shop_order SET payment_expire_time=NOW()-INTERVAL 1 MINUTE WHERE id=?", expired.Id); err != nil {
		t.Fatal(err)
	}
	if _, err := orders.UpdateStatus(ctx, paid.Id, model.OrderStatusPaid); err != nil {
		t.Fatal(err)
	}
	if err := orders.ClosePending(ctx, paid.Id, "991120"); !errors.Is(err, common.ErrOrderStatusConflict) {
		t.Fatalf("paid order cancellation: %v", err)
	}
	release := &testReleases{failed: expired.RequestId}
	service := &svc.ServiceContext{DB: db, ShopOrderModel: orders, CatalogClient: release}
	maintainPendingOrders(ctx, service)
	releases, err := orders.FindStockReleases(ctx, 100)
	if err != nil || len(releases) != 1 || releases[0].RequestID != expired.RequestId {
		t.Fatalf("release awaiting retry: %+v %v", releases, err)
	}
	release.failed = ""
	maintainPendingOrders(ctx, service)
	maintainPendingOrders(ctx, service)
	if !slices.Equal(release.ids, []string{cancelled.RequestId, expired.RequestId, expired.RequestId}) {
		t.Fatalf("unexpected stock releases: %+v", release.ids)
	}
	for _, initial := range []*model.ShopOrder{cancelled, expired} {
		row, err := orders.FindOne(ctx, initial.Id)
		if err != nil || row.Status != "cancelled" || row.PaymentExpireTime == "" || row.FreightAmount != 10 {
			t.Fatalf("closed order: %+v %v", row, err)
		}
	}
}

type testReleases struct {
	failed string
	ids    []string
}

func (*testReleases) ReserveCart(context.Context, string, []*catalog.CartLine) (*catalog.ReserveCartResp, error) {
	return nil, errors.New("unexpected reservation")
}
func (r *testReleases) ReleaseCart(_ context.Context, id string) error {
	r.ids = append(r.ids, id)
	if id == r.failed {
		return errors.New("catalog temporarily unavailable")
	}
	return nil
}

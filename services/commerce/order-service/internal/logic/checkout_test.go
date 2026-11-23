package logic

import (
	"context"
	"errors"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/askxuan/common"
	"github.com/askxuan/common/catalogtranslation"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/common/rpc/catalog"
	"github.com/askxuan/order-service/internal/model"
	"github.com/askxuan/order-service/internal/mq"
	"github.com/askxuan/order-service/internal/svc"
	"github.com/askxuan/order-service/internal/types"
	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/stores/redis"
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

func TestEnglishShopCheckoutSnapshotsAndReleasesStaleCart(t *testing.T) {
	fixtureDSN, serviceDSN := os.Getenv("CATALOG_TEST_FIXTURE_DSN"), os.Getenv("CATALOG_TEST_ORDER_DSN")
	if fixtureDSN == "" || serviceDSN == "" || os.Getenv("CATALOG_TEST_ISOLATED") != "1" {
		t.Skip("requires a disposable migrated MySQL and the order service account")
	}
	fixtures, db := sqlx.NewMysql(fixtureDSN), sqlx.NewMysql(serviceDSN)
	ctx := context.WithValue(context.Background(), middleware.CtxKeyUserID, int64(991123))
	productNo := strings.ReplaceAll(uuid.NewString(), "-", "")
	created, err := fixtures.ExecCtx(ctx, `INSERT INTO askxuan_product.product
		(product_no,name,description,status,is_experience,price,stock) VALUES (?,'青木手串','本地测试商品','on_shelf',1,48.50,8)`, productNo)
	if err != nil {
		t.Fatal(err)
	}
	productID, err := created.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	id := strconv.FormatInt(productID, 10)
	key := "shop-product:" + id
	t.Cleanup(func() {
		_, _ = fixtures.ExecCtx(ctx, "DELETE FROM askxuan_ai.catalog_translation WHERE kind='shop-product' AND resource_id=?", id)
		_, _ = fixtures.ExecCtx(ctx, "DELETE FROM askxuan_product.product WHERE id=?", productID)
	})
	source, err := catalogtranslation.Load(ctx, fixtures, "shop-product", id)
	if err != nil {
		t.Fatal(err)
	}
	english := `{"title":"Greenwood Prayer Beads","description":"A wooden bracelet for daily wear."}`
	if _, err := fixtures.ExecCtx(ctx, `INSERT INTO askxuan_ai.catalog_translation
		(kind,resource_id,source_revision,status,fields,published_fields,published_source_revision)
		VALUES ('shop-product',?,?,'published',?,?,?)`, id, source.Revision, english, english, source.Revision); err != nil {
		t.Fatal(err)
	}
	reviewed, err := catalogtranslation.LookupSession(ctx, db, "shop-product", []string{id})
	if err != nil || !reviewed[id].TransactionReady {
		t.Fatalf("expected current English from the scoped order service account: %+v %v", reviewed, err)
	}
	reservation := &catalogFixture{quote: &catalog.ReserveCartResp{
		TotalAmount: 48.5, Items: []*catalog.CatalogQuote{{
			ProductId: productID, ProductName: "青木手串", UnitPrice: 48.5, Quantity: 1, IsExperience: true,
		}},
	}}
	service := &svc.ServiceContext{
		DB: db, ShopOrderModel: model.NewShopOrderModel(db), ShopOrderItemModel: model.NewShopOrderItemModel(db),
		ShopOrderLogisticsModel: model.NewShopOrderLogisticsModel(db), CatalogClient: reservation,
		MqProducer: mq.NewProducer("127.0.0.1", 1, "fixture", "fixture", "/"),
		Redis:      redis.New("127.0.0.1:1"),
	}
	request := types.OrderCreateReq{
		RequestId: uuid.NewString(), Items: []types.ShopOrderItem{{ProductId: productID, Quantity: 1, Price: 1}},
		CatalogRevisions: map[string]string{key: reviewed[id].Revision},
	}
	order, err := NewOrderCreateLogic(ctx, service).Create(&request)
	if err != nil || order.PayAmount != 48.5 || !common.IsExperienceOrder(order.OrderNo) {
		t.Fatalf("checkout did not use the authoritative price: %+v %v", order, err)
	}
	t.Cleanup(func() {
		_, _ = db.ExecCtx(ctx, "DELETE FROM catalog_checkout_snapshot WHERE id=?", order.Id)
		_, _ = db.ExecCtx(ctx, "DELETE FROM shop_order_item WHERE order_id=?", order.Id)
		_, _ = db.ExecCtx(ctx, "DELETE FROM shop_order WHERE id=?", order.Id)
	})
	if _, err := fixtures.ExecCtx(ctx, "UPDATE askxuan_product.product SET description='请到店自取。' WHERE id=?", productID); err != nil {
		t.Fatal(err)
	}
	stale := request
	stale.RequestId = uuid.NewString()
	if _, err := NewOrderCreateLogic(ctx, service).Create(&stale); !errors.Is(err, catalogtranslation.ErrCheckoutRevision) {
		t.Fatalf("stale English checkout: %v", err)
	}
	if !slices.Equal(reservation.ids, []string{stale.RequestId}) {
		t.Fatalf("stale cart inventory was not released: %+v", reservation.ids)
	}
	detail, err := NewOrderDetailLogic(ctx, service).Detail(&types.OrderDetailReq{Id: order.Id, IncludeCatalogSnapshot: true})
	if err != nil || detail.CatalogSnapshot[key].Fields["description"] != "A wooden bracelet for daily wear." {
		t.Fatalf("order lost the terms originally displayed: %+v %v", detail, err)
	}
	foreign := context.WithValue(ctx, middleware.CtxKeyUserID, int64(991124))
	if _, err := NewOrderDetailLogic(foreign, service).Detail(&types.OrderDetailReq{Id: order.Id, IncludeCatalogSnapshot: true}); !errors.Is(err, common.ErrForbidden) {
		t.Fatalf("another customer could read the order snapshot: %v", err)
	}
}

type catalogFixture struct {
	testReleases
	quote *catalog.ReserveCartResp
}

func (r *catalogFixture) ReserveCart(context.Context, string, []*catalog.CartLine) (*catalog.ReserveCartResp, error) {
	return r.quote, nil
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

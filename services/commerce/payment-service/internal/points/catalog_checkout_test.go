package points

import (
	"context"
	"errors"
	"os"
	"strconv"
	"testing"

	"github.com/askxuan/common/catalogtranslation"
	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestEnglishRedemptionRetainsConfirmedTerms(t *testing.T) {
	fixtureDSN, serviceDSN := os.Getenv("CATALOG_TEST_FIXTURE_DSN"), os.Getenv("CATALOG_TEST_POINTS_DSN")
	if fixtureDSN == "" || serviceDSN == "" || os.Getenv("CATALOG_TEST_ISOLATED") != "1" {
		t.Skip("requires a disposable migrated MySQL and the payment service account")
	}
	ctx := context.Background()
	fixtures := sqlx.NewMysql(fixtureDSN)
	db := sqlx.NewMysql(serviceDSN)
	store := Store{DB: db}

	user := "catalog-test-" + uuid.NewString()
	product, err := store.SaveProduct(ctx, Product{
		Name: "随身平安卡", Category: "平安礼物", Description: "可随身携带。",
		PointsPrice: 120, Stock: 5, Status: "on_sale",
	})
	if err != nil {
		t.Fatal(err)
	}
	id := strconv.FormatInt(product.ID, 10)
	key := "points-product:" + id
	t.Cleanup(func() {
		for _, stmt := range []string{
			"DELETE snap FROM catalog_checkout_snapshot snap JOIN points_order o ON o.id=snap.id WHERE o.user_id=?",
			"DELETE FROM points_order WHERE user_id=?",
			"DELETE FROM points_ledger WHERE user_id=?",
			"DELETE FROM points_account WHERE user_id=?",
		} {
			_, _ = db.ExecCtx(ctx, stmt, user)
		}
		_, _ = db.ExecCtx(ctx, "DELETE FROM points_product WHERE id=?", product.ID)
		_, _ = fixtures.ExecCtx(ctx, "DELETE FROM askxuan_ai.catalog_translation WHERE kind='points-product' AND resource_id=?", id)
	})
	if _, err := db.ExecCtx(ctx, "INSERT INTO points_account (user_id,balance) VALUES (?,360)", user); err != nil {
		t.Fatal(err)
	}
	source, err := catalogtranslation.Load(ctx, fixtures, "points-product", id)
	if err != nil {
		t.Fatal(err)
	}
	english := `{"title":"Peace card","subtitle":"Gifts for peace","description":"Carry it with you."}`
	if _, err := fixtures.ExecCtx(ctx, `INSERT INTO askxuan_ai.catalog_translation
		(kind,resource_id,source_revision,status,fields,published_fields,published_source_revision)
		VALUES ('points-product',?,?,'published',?,?,?)`, id, source.Revision, english, english, source.Revision); err != nil {
		t.Fatal(err)
	}
	translations, err := catalogtranslation.LookupSession(ctx, db, "points-product", []string{id})
	if err != nil || !translations[id].TransactionReady {
		t.Fatalf("current English should be purchasable: %+v %v", translations, err)
	}
	request := RedeemRequest{ProductID: product.ID, Quantity: 1, ExpectedPrice: 120,
		RequestKey: uuid.NewString(), Receiver: "Fixture", Mobile: "123456789", Address: "Fixture address",
		CatalogRevisions: map[string]string{key: translations[id].Revision}}
	order, err := store.Redeem(ctx, user, request)
	if err != nil || order.ID == 0 {
		t.Fatalf("redemption failed to retain approved terms: %+v %v", order, err)
	}

	edited := `{"title":"Peace card","subtitle":"Gifts for peace","description":"A portable keepsake."}`
	if _, err := fixtures.ExecCtx(ctx, `UPDATE askxuan_ai.catalog_translation SET published_fields=?
		WHERE kind='points-product' AND resource_id=?`, edited, id); err != nil {
		t.Fatal(err)
	}
	next := request
	next.RequestKey = uuid.NewString()
	if _, err := store.Redeem(ctx, user, next); !errors.Is(err, catalogtranslation.ErrCheckoutRevision) {
		t.Fatalf("checkout accepted older English terms after an English-only edit: %v", err)
	}
	if _, err := db.ExecCtx(ctx, "UPDATE points_product SET description='请到现场领取。' WHERE id=?", product.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Redeem(ctx, user, next); !errors.Is(err, catalogtranslation.ErrCheckoutRevision) {
		t.Fatalf("checkout accepted English after the Chinese terms changed: %v", err)
	}
	retry, err := store.Redeem(ctx, user, request)
	if err != nil || retry.ID != order.ID {
		t.Fatalf("an accepted request should still be idempotent: %+v %v", retry, err)
	}
	account, err := store.Account(ctx, user)
	if err != nil || account.Balance != 240 {
		t.Fatalf("a stale attempt spent points: %+v %v", account, err)
	}
	next.CatalogRevisions = nil
	chineseOrder, err := store.Redeem(ctx, user, next)
	if err != nil || chineseOrder.ID == order.ID {
		t.Fatalf("the original Chinese checkout should remain available: %+v %v", chineseOrder, err)
	}
	snapshots, err := catalogtranslation.LoadCheckoutSnapshots(ctx, db, []int64{order.ID, chineseOrder.ID})
	if err != nil || len(snapshots) != 1 || snapshots[order.ID][key].Fields["description"] != "Carry it with you." {
		t.Fatalf("stored snapshot changed with catalog edits: %+v %v", snapshots, err)
	}
}

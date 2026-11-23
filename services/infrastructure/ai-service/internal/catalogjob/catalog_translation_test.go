package catalogjob

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/askxuan/ai-service/internal/provider"
	catalog "github.com/askxuan/common/catalogtranslation"
	"github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type textFixture struct {
	provider.Mock
	answer func(provider.Request) string
}

func (textFixture) Name() string                     { return "text-fixture" }
func (textFixture) ModelFor(provider.Request) string { return "text-only" }
func (f textFixture) Complete(_ context.Context, req provider.Request) (*provider.Response, error) {
	return &provider.Response{Content: f.answer(req), FinishReason: "stop"}, nil
}

func TestCatalogTranslationMasksTermsAndValidatesResponse(t *testing.T) {
	src := map[string]string{"title": "紫檀手珠", "description": "共108颗，联系 review@example.invalid。"}
	fake := textFixture{answer: func(req provider.Request) string {
		if !req.JSONMode || req.Model != "text-only" || len(req.Messages) != 1 {
			t.Fatalf("unexpected text request: %#v", req)
		}
		input := req.Messages[0].Content
		if strings.Contains(input, "108") || strings.Contains(input, "review@example.invalid") ||
			!strings.Contains(input, "{{T0}}") || !strings.Contains(input, "{{T1}}") {
			t.Fatalf("protected content sent to model: %s", input)
		}
		return `{"title":"Sandalwood prayer beads","description":"{{T0}} beads. Contact {{T1}}."}`
	}}
	got, err := Translate(context.Background(), fake, provider.NewCatalog(fake), 1200, src)
	if err != nil || got["description"] != "108 beads. Contact review@example.invalid." {
		t.Fatalf("translation=%v err=%v", got, err)
	}
	fake.answer = func(provider.Request) string { return `{"title":"Sandalwood beads","description":"Contact {{T1}}."}` }
	if _, err := Translate(context.Background(), fake, provider.NewCatalog(fake), 1200, src); err == nil {
		t.Fatal("accepted dropped numerical term")
	}
}

func TestMySQLCatalogTranslationWorkflow(t *testing.T) {
	rootDSN, appDSN := os.Getenv("ASKXUAN_CATALOG_ROOT_TEST_DSN"), os.Getenv("ASKXUAN_CATALOG_AI_TEST_DSN")
	if rootDSN == "" || appDSN == "" {
		t.Skip("requires a disposable loopback MySQL with the catalog migration")
	}
	for _, dsn := range []string{rootDSN, appDSN} {
		cfg, err := mysql.ParseDSN(dsn)
		if err != nil || cfg.Net != "tcp" || !strings.HasPrefix(cfg.Addr, "127.0.0.1:") || cfg.DBName != "askxuan_ai" {
			t.Fatal("only disposable loopback fixtures are supported")
		}
	}
	ctx := context.Background()
	sqlx.DisableLog()
	root, err := sql.Open("mysql", rootDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	db := sqlx.NewMysql(appDSN)
	app, err := db.RawDB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { app.Close() })
	store := Store{DB: db}
	exec := func(statement string, args ...any) {
		t.Helper()
		if _, err := root.ExecContext(ctx, statement, args...); err != nil {
			t.Fatal(err)
		}
	}
	const kind, id, templateID = "shop-product", "8671971", 8671971
	for catalogKind, catalogID := range map[string]string{
		"shop-product": id, "points-product": id, "reward-campaign": id, "reward-prize": id, "diy-material": id,
		"diy-design": id, "temple": "T001", "temple-service": id, "master": "M001", "belief": "han_buddhism",
	} {
		if _, err := catalog.Lookup(ctx, app, catalogKind, []string{catalogID}); err != nil {
			t.Fatalf("%s published projection: %v", catalogKind, err)
		}
		source, err := catalog.Load(ctx, db, catalogKind, catalogID)
		if err != nil && !errors.Is(err, sqlx.ErrNotFound) {
			t.Fatalf("%s Chinese projection: %v", catalogKind, err)
		}
		if err == nil && (catalogKind == "temple" || catalogKind == "master") {
			canonical, err := catalog.Load(ctx, db, catalogKind, source.ID)
			if err != nil || canonical.Revision != source.Revision {
				t.Fatalf("%s code and ID must share a source revision: %v", catalogKind, err)
			}
		}
	}
	exec(`INSERT INTO askxuan_logistics.freight_template(id,name,type,free_shipping,config)
		VALUES(?, '目录测试规则','by_piece',0,?)`, templateID, `{"first":1,"firstFee":5,"next":1,"nextFee":2}`)
	t.Cleanup(func() { exec(`DELETE FROM askxuan_logistics.freight_template WHERE id=?`, templateID) })
	exec(`INSERT INTO askxuan_product.product(id,product_no,name,description,status,freight_template_id)
		VALUES(?,'CATALOG-EN-TEST','紫檀手珠','共108颗','on_shelf',?)`, id, templateID)
	t.Cleanup(func() {
		exec(`DELETE FROM catalog_translation WHERE kind=? AND resource_id=?`, kind, id)
		exec(`DELETE FROM askxuan_product.product WHERE id=?`, id)
	})
	manage := func(enqueue bool, edit *Edit) Draft {
		t.Helper()
		out, err := store.Manage(ctx, kind, id, "", 50, enqueue, edit)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	lookup := func() (catalog.Published, bool) {
		t.Helper()
		items, err := catalog.LookupSession(ctx, db, kind, []string{id})
		if err != nil {
			t.Fatal(err)
		}
		row, found := items[id]
		return row, found
	}
	if draft := manage(false, nil); draft.Status != "missing" || !draft.PublicEligible {
		t.Fatalf("unexpected initial status: %+v", draft)
	}
	queued := manage(true, nil)
	if queued.Status != "queued" || manage(true, nil).Revision != queued.Revision {
		t.Fatalf("queueing should be idempotent: %+v", queued)
	}
	fake := textFixture{answer: func(provider.Request) string {
		return `{"title":"Sandalwood beads","description":"{{T0}} beads"}`
	}}
	translate := func(ctx context.Context, source map[string]string) (map[string]string, error) {
		return Translate(ctx, fake, provider.NewCatalog(fake), 1200, source)
	}
	if err := store.processNext(ctx, translate); err != nil {
		t.Fatal(err)
	}
	review := manage(false, nil)
	if review.Status != "review" || review.Fields["description"] != "108 beads" || !review.ReviewRequired {
		t.Fatalf("unexpected reviewed draft: %+v", review)
	}
	if _, found := lookup(); found {
		t.Fatal("machine draft appeared in the customer catalog")
	}
	published := manage(false, &Edit{Revision: review.Revision, Fields: review.Fields, Publish: true})
	before, found := lookup()
	if published.Status != "published" || published.ReviewRequired || !found || !before.TransactionReady {
		t.Fatalf("not published: %+v %+v", published, before)
	}
	review.Fields["title"] = "Handmade sandalwood beads"
	edited := manage(false, &Edit{Revision: published.Revision, Fields: review.Fields})
	if old, _ := lookup(); old.Revision != before.Revision {
		t.Fatal("saving an English draft replaced the approved public version")
	}
	approved := manage(false, &Edit{Revision: edited.Revision, Fields: review.Fields, Publish: true})
	after, _ := lookup()
	if before.Revision == after.Revision {
		t.Fatal("English edits must change the public transaction revision")
	}
	exec("UPDATE askxuan_product.product SET status='off_shelf' WHERE id=?", id)
	if _, found := lookup(); found {
		t.Fatal("English remained visible after source delist")
	}
	exec("UPDATE askxuan_product.product SET status='on_shelf' WHERE id=?", id)
	exec("UPDATE askxuan_logistics.freight_template SET config=? WHERE id=?", `{"first":2,"firstFee":4,"next":1,"nextFee":1}`, templateID)
	if _, found := lookup(); found {
		t.Fatal("stale delivery terms remained visible after Chinese delivery change")
	}
	if _, err := store.Manage(ctx, kind, id, "", 50, false, &Edit{Revision: approved.Revision, Fields: review.Fields, Publish: true}); !errors.Is(err, errConflict) {
		t.Fatalf("stale review accepted: %v", err)
	}
	manage(true, nil)
	if err := store.processNext(ctx, func(context.Context, map[string]string) (map[string]string, error) {
		t.Fatal("delivery policy change with unchanged text consumed a model call")
		return nil, nil
	}); err != nil {
		t.Fatal(err)
	}
	deliveryReview := manage(false, nil)
	if deliveryReview.Status != "review" || deliveryReview.Fields["title"] != "Handmade sandalwood beads" {
		t.Fatalf("delivery policy change did not preserve English human wording for review: %+v", deliveryReview)
	}
	if _, found := lookup(); found {
		t.Fatal("carried English became public before the new delivery policy was reviewed")
	}
	manage(false, &Edit{Revision: deliveryReview.Revision, Fields: deliveryReview.Fields, Publish: true})

	exec("UPDATE askxuan_product.product SET description='共有120颗' WHERE id=?", id)
	manage(true, nil)
	if err := store.processNext(ctx, func(_ context.Context, source map[string]string) (map[string]string, error) {
		if len(source) != 1 || source["description"] != "共有120颗" {
			t.Fatalf("unexpected changed fields: %v", source)
		}
		return map[string]string{"description": "120 beads"}, nil
	}); err != nil {
		t.Fatal(err)
	}
	changed := manage(false, nil)
	if changed.Status != "review" || changed.Fields["title"] != "Handmade sandalwood beads" || changed.Fields["description"] != "120 beads" {
		t.Fatalf("translation did not merge reviewed title with changed description: %+v", changed)
	}
	manage(false, &Edit{Revision: changed.Revision, Fields: changed.Fields, Publish: true})
	exec("UPDATE askxuan_product.product SET description='共110颗' WHERE id=?", id)
	manage(true, nil)
	err = store.processNext(ctx, func(_ context.Context, _ map[string]string) (map[string]string, error) {
		active := manage(false, nil)
		manage(false, &Edit{Revision: active.Revision, Fields: map[string]string{"title": "Handmade sandalwood beads", "description": "110 beads"}, Publish: true})
		return map[string]string{"description": "Late machine result"}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if item, _ := lookup(); item.Fields["title"] != "Handmade sandalwood beads" || item.Fields["description"] != "110 beads" {
		t.Fatalf("late worker replaced a human edit: %+v", item)
	}
	exec("INSERT INTO askxuan_product.product_sku(product_id,spec_name,spec_value) VALUES(?,'颜色','紫色')", id)
	t.Cleanup(func() { exec("DELETE FROM askxuan_product.product_sku WHERE product_id=?", id) })
	if item, found := lookup(); !found || item.TransactionReady {
		t.Fatalf("catalog should show English for a product that has untranslated SKU choices: %+v", item)
	}

	if _, err := store.Manage(ctx, "temple", "T001", "not-the-owner", 51, false, nil); err == nil {
		t.Fatal("cross-temple admin reached another temple's draft")
	}

	const designID = "8671972"
	exec(`INSERT INTO askxuan_diy.diy_design(id,design_no,user_id,name,status,publication_id)
		VALUES(?,'CATALOG-EN-DESIGN','catalog-fixture-owner','作者未发布更改','public',?)`, designID, designID)
	t.Cleanup(func() { exec("DELETE FROM askxuan_diy.diy_design WHERE id=?", designID) })
	exec(`INSERT INTO askxuan_diy.diy_order(id,order_no,user_id,design_id,payment_status,status)
		VALUES(?,'CATALOG-EN-DESIGN-ORDER','catalog-fixture-owner',?,'success','completed')`, designID, designID)
	t.Cleanup(func() { exec("DELETE FROM askxuan_diy.diy_order WHERE id=?", designID) })
	exec(`INSERT INTO askxuan_diy.diy_design_publication(id,design_id,design_revision,creator_id,order_id,composition_hash,design_snapshot,pricing_snapshot,material_fee)
		VALUES(?,?,1,'catalog-fixture-owner',?,REPEAT('0',64),?,'{}',18)`, designID, designID, designID,
		`{"id":8671972,"userId":"catalog-fixture-owner","name":"公开手珠","description":"展示说明","designData":"内部配置"}`)
	t.Cleanup(func() { exec("DELETE FROM askxuan_diy.diy_design_publication WHERE id=?", designID) })
	exec(`INSERT INTO askxuan_payment.payment(id,payment_no,user_id,order_type,order_no,channel,status)
		VALUES(?,'CATALOG-EN-DESIGN-PAYMENT','catalog-fixture-owner','diy_order','CATALOG-EN-DESIGN-ORDER','mock','success')`, designID)
	t.Cleanup(func() { exec("DELETE FROM askxuan_payment.payment WHERE id=?", designID) })
	design, err := store.Manage(ctx, "diy-design", designID, "", 50, false, nil)
	if err != nil || !design.PublicEligible || len(design.Source) != 2 || design.Source["title"] != "公开手珠" {
		t.Fatalf("DIY source was not limited to the current public snapshot: %+v %v", design, err)
	}
	t.Cleanup(func() { exec("DELETE FROM catalog_translation WHERE kind='diy-design' AND resource_id=?", designID) })
	_, err = store.Manage(ctx, "diy-design", designID, "", 50, false, &Edit{
		Revision: design.Revision, Fields: map[string]string{"title": "Public prayer beads", "description": "Design details"}, Publish: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	designs, err := catalog.Lookup(ctx, app, "diy-design", []string{designID})
	if err != nil || designs[designID].Fields["title"] == "" || designs[designID].TransactionReady {
		t.Fatalf("DIY browse-only translation was not available: %+v %v", designs, err)
	}
	exec(`INSERT INTO askxuan_payment.refund(refund_no,payment_id,status)
		VALUES('CATALOG-EN-DESIGN-REFUND',?,'pending')`, designID)
	t.Cleanup(func() { exec("DELETE FROM askxuan_payment.refund WHERE refund_no='CATALOG-EN-DESIGN-REFUND'") })
	designs, err = catalog.Lookup(ctx, app, "diy-design", []string{designID})
	if err != nil || len(designs) != 0 {
		t.Fatalf("English copy became visible despite an invalid order: %+v %v", designs, err)
	}
}

package rewards

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/askxuan/common/catalogtranslation"
	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestEnglishPoolConfirmsTheSelectedPrize(t *testing.T) {
	fixtureDSN, serviceDSN := os.Getenv("CATALOG_TEST_FIXTURE_DSN"), os.Getenv("CATALOG_TEST_REWARDS_DSN")
	if fixtureDSN == "" || serviceDSN == "" || os.Getenv("CATALOG_TEST_ISOLATED") != "1" {
		t.Skip("requires a disposable migrated MySQL and the marketing service account")
	}
	ctx := context.Background()
	fixtures := sqlx.NewMysql(fixtureDSN)
	db, err := sql.Open("mysql", serviceDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	store := Store{DB: db}
	now := time.Now().Unix()
	campaign, err := store.Save(ctx, Campaign{
		Title: "积分奖池", Kind: "pool", Description: "两个奖品独立开奖。", Rules: "每个奖品可参与一次。",
		Budget: 200, PointsCost: 10, StartsAt: now - 60, EndsAt: now + 3600,
		Prizes: []Prize{
			{Name: "第一份奖品", Kind: "physical", Value: 100, Quantity: 1, Capacity: 4},
			{Name: "第二份奖品", Kind: "physical", Value: 100, Quantity: 1, Capacity: 4},
		},
	}, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	user := "catalog-pool-" + uuid.NewString()
	cid := strconv.FormatInt(campaign.ID, 10)
	firstID, secondID := campaign.Prizes[0].ID, campaign.Prizes[1].ID
	t.Cleanup(func() {
		_, _ = fixtures.ExecCtx(ctx, "DELETE snap FROM askxuan_marketing.catalog_checkout_snapshot snap JOIN askxuan_marketing.reward_entry e ON e.id=snap.id WHERE e.campaign_id=?", campaign.ID)
		for _, table := range []string{"reward_entry", "reward_prize", "reward_audit"} {
			_, _ = fixtures.ExecCtx(ctx, "DELETE FROM askxuan_marketing."+table+" WHERE campaign_id=?", campaign.ID)
		}
		_, _ = fixtures.ExecCtx(ctx, "DELETE FROM askxuan_marketing.reward_campaign WHERE id=?", campaign.ID)
		_, _ = fixtures.ExecCtx(ctx, "DELETE FROM askxuan_ai.catalog_translation WHERE (kind='reward-campaign' AND resource_id=?) OR (kind='reward-prize' AND resource_id IN (?,?))", cid, strconv.FormatInt(firstID, 10), strconv.FormatInt(secondID, 10))
		_, _ = fixtures.ExecCtx(ctx, "DELETE FROM askxuan_payment.points_ledger WHERE user_id=?", user)
		_, _ = fixtures.ExecCtx(ctx, "DELETE FROM askxuan_payment.points_account WHERE user_id=?", user)
	})
	if err := store.Publish(ctx, campaign.ID, "fixture"); err != nil {
		t.Fatal(err)
	}
	if _, err := fixtures.ExecCtx(ctx, "INSERT INTO askxuan_payment.points_account(user_id,balance) VALUES(?,30)", user); err != nil {
		t.Fatal(err)
	}
	expected := map[string]string{}
	for _, row := range []struct {
		kind, id, english string
	}{
		{"reward-campaign", cid, `{"title":"Points Pool","description":"Each prize is drawn independently.","rules":"Join each prize once.","prizeName":"First prize"}`},
		{"reward-prize", strconv.FormatInt(firstID, 10), `{"title":"First prize"}`},
	} {
		source, err := catalogtranslation.Load(ctx, fixtures, row.kind, row.id)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := fixtures.ExecCtx(ctx, `INSERT INTO askxuan_ai.catalog_translation
			(kind,resource_id,source_revision,status,fields,published_fields,published_source_revision)
			VALUES (?,?,?,'published',?,?,?)`, row.kind, row.id, source.Revision, row.english, row.english, source.Revision); err != nil {
			t.Fatal(err)
		}
		published, err := catalogtranslation.Lookup(ctx, db, row.kind, []string{row.id})
		if err != nil || !published[row.id].TransactionReady {
			t.Fatalf("reviewed %s missing: %+v %v", row.kind, published, err)
		}
		expected[row.kind+":"+row.id] = published[row.id].Revision
	}
	if _, err := store.JoinPrize(ctx, campaign.ID, user, 10, "", secondID, expected); !errors.Is(err, catalogtranslation.ErrCheckoutRevision) {
		t.Fatalf("a different prize accepted these English terms: %v", err)
	}
	entry, err := store.JoinPrize(ctx, campaign.ID, user, 10, "", firstID, expected)
	if err != nil || entry.PrizeID != firstID {
		t.Fatalf("the reviewed prize was not accepted: %+v %v", entry, err)
	}
	if _, err := store.JoinPrize(ctx, campaign.ID, user, 10, "", secondID); err != nil {
		t.Fatalf("the second prize should remain available in Chinese: %v", err)
	}
	reviewed, err := catalogtranslation.LoadCheckoutSnapshots(ctx, sqlx.NewSqlConnFromDB(db), []int64{entry.ID})
	if err != nil || reviewed[entry.ID]["reward-campaign:"+cid].Fields["rules"] != "Join each prize once." {
		t.Fatalf("accepted campaign rules were lost: %+v %v", reviewed, err)
	}
	var balance int
	if err := db.QueryRowContext(ctx, "SELECT balance FROM askxuan_payment.points_account WHERE user_id=?", user).Scan(&balance); err != nil || balance != 10 {
		t.Fatalf("successful joins should debit exactly 20 points: %d %v", balance, err)
	}
}

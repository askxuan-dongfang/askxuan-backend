package rewards

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/go-sql-driver/mysql"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func fixture() Campaign {
	now := time.Now().Unix()
	return Campaign{Title: "测试积分好礼", Kind: "pool", PrizeName: "实物礼品", Description: "隔离验收", Rules: "消耗积分参与，平台包邮", PrizeValue: 10000, Budget: 15000, PointsCost: 10, PrizeQuantity: 1, Capacity: 100, StartsAt: now - 10, EndsAt: now + 3600}
}
func TestValidation(t *testing.T) {
	c := fixture()
	if e := Validate(c, time.Now().Unix()); e != nil {
		t.Fatal(e)
	}
	for _, mutate := range []func(*Campaign){func(c *Campaign) { c.PointsCost = 0 }, func(c *Campaign) { c.PointsCost = -1 }, func(c *Campaign) { c.PointsCost = 100000001 }, func(c *Campaign) { c.Budget = 9999 }, func(c *Campaign) { c.PrizeQuantity = 101 }, func(c *Campaign) { c.EndsAt = c.StartsAt }, func(c *Campaign) { c.Image = "javascript:alert(1)" }, func(c *Campaign) { c.Kind = "points" }, func(c *Campaign) { c.Capacity = 100001 }} {
		v := c
		mutate(&v)
		if Validate(v, time.Now().Unix()) == nil {
			t.Fatal("invalid campaign accepted")
		}
	}
}

type failedReader struct{}

func (failedReader) Read([]byte) (int, error) { return 0, errors.New("entropy unavailable") }
func TestUniform(t *testing.T) {
	s := Store{}
	for _, n := range []int{1, 2, 100, 100000} {
		for i := 0; i < 100; i++ {
			v, e := s.uniform(n)
			if e != nil || v < 0 || v >= n {
				t.Fatalf("n=%d v=%d err=%v", n, v, e)
			}
		}
	}
	if _, e := (Store{Random: failedReader{}}).uniform(100); e == nil {
		t.Fatal("entropy error ignored")
	}
}
func TestMySQLRewards(t *testing.T) {
	dsn := os.Getenv("REWARDS_TEST_DSN")
	if dsn == "" {
		t.Skip("isolated REWARDS_TEST_DSN required")
	}
	if !strings.Contains(dsn, "/askxuan_rewards_test") {
		t.Fatal("must use isolated askxuan_rewards_test database")
	}
	db, e := sql.Open("mysql", dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	db.SetMaxOpenConns(16)
	ctx := context.Background()
	s := Store{DB: db}
	schema, e := os.ReadFile("../../../../../scripts/db/20260909_free_rewards.sql")
	if e != nil {
		t.Fatal(e)
	}
	schema = []byte(strings.ReplaceAll(string(schema), "USE askxuan_marketing;", ""))
	pointsSQL, e := os.ReadFile("../../../../../scripts/db/20260907_points_mall.sql")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = db.Exec("CREATE DATABASE IF NOT EXISTS askxuan_payment"); e != nil {
		t.Fatal(e)
	}
	pointsLines := []string{}
	for _, line := range strings.Split(string(pointsSQL), "\n") {
		if !strings.HasPrefix(strings.TrimSpace(line), "--") {
			pointsLines = append(pointsLines, line)
		}
	}
	for _, q := range strings.Split(strings.Join(pointsLines, "\n"), ";") {
		if strings.TrimSpace(q) != "" {
			if _, e = db.Exec(q); e != nil {
				t.Fatal(e)
			}
		}
	}
	migration, e := os.ReadFile("../../../../../scripts/db/20260910_points_rewards.sql")
	if e != nil {
		t.Fatal(e)
	}
	schema = append(schema, []byte(strings.ReplaceAll(string(migration), "USE askxuan_marketing;", ""))...)
	for pass := 0; pass < 2; pass++ {
		for _, q := range strings.Split(string(schema), ";") {
			if strings.TrimSpace(q) != "" {
				if _, e := db.Exec(q); e != nil {
					t.Fatal(e)
				}
			}
		}
	}
	must := func(e error) {
		t.Helper()
		if e != nil {
			t.Fatal(e)
		}
	}
	fund := func(user string, amount int64) {
		t.Helper()
		_, e := db.Exec("INSERT INTO askxuan_payment.points_account(user_id,balance) VALUES(?,?) ON DUPLICATE KEY UPDATE balance=VALUES(balance)", user, amount)
		must(e)
	}
	for i := 0; i < 100; i++ {
		fund(fmt.Sprintf("user-%d", i), 1000)
	}
	for i := 0; i < 20; i++ {
		fund(fmt.Sprintf("wheel-%d", i), 1000)
	}
	for _, user := range []string{"underfilled-0", "underfilled-1", "entropy-user", "entropy-1", "entropy-2"} {
		fund(user, 1000)
	}
	create := func(c Campaign) Campaign {
		t.Helper()
		v, e := s.Save(ctx, c, "admin")
		must(e)
		must(s.Publish(ctx, v.ID, "admin"))
		return v
	}
	expire := func(id int64) {
		_, e := db.Exec("UPDATE reward_campaign SET ends_at=? WHERE id=?", time.Now().Unix()-1, id)
		must(e)
	}
	t.Run("one_user_one_code_and_deadline_100_to_1", func(t *testing.T) {
		c := create(fixture())
		var wg sync.WaitGroup
		out := make(chan Entry, 24)
		errs := make(chan error, 24)
		for i := 0; i < 24; i++ {
			wg.Add(1)
			go func() { defer wg.Done(); v, e := s.Join(ctx, c.ID, "user-0", 10); out <- v; errs <- e }()
		}
		wg.Wait()
		close(out)
		close(errs)
		for e := range errs {
			must(e)
		}
		id := int64(0)
		for v := range out {
			if id == 0 {
				id = v.ID
			}
			if v.ID != id {
				t.Fatal("duplicate participation")
			}
		}
		for i := 1; i < 100; i++ {
			_, e := s.Join(ctx, c.ID, fmt.Sprintf("user-%d", i), 10)
			must(e)
		}
		if _, e := s.Join(ctx, c.ID, "extra", 10); !errors.Is(e, ErrClosed) {
			t.Fatalf("over capacity: %v", e)
		}
		if e = s.Draw(ctx, c.ID); !errors.Is(e, ErrClosed) {
			t.Fatalf("early draw %v", e)
		}
		if e = s.Cancel(ctx, c.ID, "admin", "取消理由"); !errors.Is(e, ErrConflict) {
			t.Fatal("cancelled participated campaign")
		}
		c.Version = 102
		if _, e = s.Save(ctx, c, "admin"); !errors.Is(e, ErrConflict) {
			t.Fatal("published rules changed")
		}
		expire(c.ID)
		for i := 0; i < 8; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if e := s.Draw(ctx, c.ID); e != nil {
					t.Error(e)
				}
			}()
		}
		wg.Wait()
		d, e := s.Detail(ctx, c.ID, "user-0", false)
		must(e)
		if len(d.Winners) != 1 || d.Campaign.ParticipantCount != 100 || d.Campaign.Status != "drawn" || len(d.Campaign.PoolDigest) != 64 {
			t.Fatalf("bad result %+v", d.Campaign)
		}
		original, e := s.Join(ctx, c.ID, "user-0", 10)
		must(e)
		var balance, debits int64
		must(db.QueryRow("SELECT balance FROM askxuan_payment.points_account WHERE user_id='user-0'").Scan(&balance))
		must(db.QueryRow("SELECT COUNT(*) FROM askxuan_payment.points_ledger WHERE user_id='user-0' AND kind='reward_pool'").Scan(&debits))
		if balance != 990 || debits != 1 || original.PointsSpent != 10 {
			t.Fatalf("duplicate debit balance=%d debits=%d entry=%+v", balance, debits, original)
		}
		if original.ID != id {
			t.Fatal("late retry changed entry")
		}
		if _, e = s.Join(ctx, c.ID, "late-user", 10); !errors.Is(e, ErrClosed) {
			t.Fatal("late entry accepted")
		}
		var winner string
		must(db.QueryRow("SELECT user_id FROM reward_entry WHERE id=?", d.Winners[0].ID).Scan(&winner))
		orders, e := s.Orders(ctx, winner, false, 1, "")
		must(e)
		if len(orders) != 1 {
			t.Fatal("wrong order count")
		}
		o := orders[0]
		a := OrderAction{Receiver: "验收用户", Mobile: "13800000000", Address: "上海市测试路 100 号"}
		if e = s.OrderAction(ctx, o.ID, "intruder", "claim", a); !errors.Is(e, ErrNotFound) {
			t.Fatal("foreign claim")
		}
		if e = s.OrderAction(ctx, o.ID, winner, "complete", OrderAction{}); !errors.Is(e, ErrConflict) {
			t.Fatal("early complete")
		}
		must(s.OrderAction(ctx, o.ID, winner, "claim", a))
		must(s.OrderAction(ctx, o.ID, winner, "claim", a))
		a.Address = "另一地址不应覆盖"
		if e = s.OrderAction(ctx, o.ID, winner, "claim", a); !errors.Is(e, ErrConflict) {
			t.Fatal("changed claimed address")
		}
		shipment := OrderAction{Carrier: "顺丰", TrackingNo: "TEST100000"}
		must(s.OrderAction(ctx, o.ID, "admin", "ship", shipment))
		must(s.OrderAction(ctx, o.ID, "admin", "ship", shipment))
		shipment.TrackingNo = "OTHER"
		if e = s.OrderAction(ctx, o.ID, "admin", "ship", shipment); !errors.Is(e, ErrConflict) {
			t.Fatal("overwrote tracking")
		}
		if e = s.OrderAction(ctx, o.ID, "intruder", "complete", OrderAction{}); !errors.Is(e, ErrNotFound) {
			t.Fatal("foreign completion")
		}
		must(s.OrderAction(ctx, o.ID, winner, "complete", OrderAction{}))
		must(s.OrderAction(ctx, o.ID, winner, "complete", OrderAction{}))
		orders, e = s.Orders(ctx, winner, false, 1, "")
		must(e)
		if orders[0].Status != "completed" || orders[0].CompletedAt == 0 {
			t.Fatal("fulfillment incomplete")
		}
		hidden, e := s.Orders(ctx, "intruder", false, 1, "")
		must(e)
		if len(hidden) != 0 {
			t.Fatal("foreign orders leaked")
		}
	})
	t.Run("zero_and_underfilled_deadline", func(t *testing.T) {
		for _, n := range []int{0, 2} {
			c := fixture()
			c.PrizeQuantity = 3
			c.Budget = 50000
			c = create(c)
			for i := 0; i < n; i++ {
				_, e := s.Join(ctx, c.ID, fmt.Sprintf("underfilled-%d", i), 10)
				must(e)
			}
			expire(c.ID)
			must(s.DrawDue(ctx))
			d, e := s.Detail(ctx, c.ID, "", false)
			must(e)
			if d.Campaign.Status != "drawn" || len(d.Winners) != n {
				t.Fatal("underfilled activity did not close")
			}
		}
	})
	t.Run("wheel_without_replacement_and_retry", func(t *testing.T) {
		c := fixture()
		c.Kind = "wheel"
		c.Capacity = 20
		c.PrizeQuantity = 3
		c.Budget = 50000
		c = create(c)
		for i := 0; i < 20; i++ {
			current, err := s.campaign(ctx, c.ID, true)
			must(err)
			if current.Phase == "exhausted" {
				break
			}
			v, e := s.Join(ctx, c.ID, fmt.Sprintf("wheel-%d", i), 10)
			must(e)
			v2, e := s.Join(ctx, c.ID, fmt.Sprintf("wheel-%d", i), 10)
			must(e)
			if v.ID != v2.ID || v.Outcome != v2.Outcome {
				t.Fatal("rerolled wheel")
			}
		}
		d, e := s.Detail(ctx, c.ID, "", false)
		must(e)
		if _, err := s.Join(ctx, c.ID, "wheel-after-exhausted", 10); !errors.Is(err, ErrClosed) {
			t.Fatalf("charged zero-chance entry: %v", err)
		}
		if d.Campaign.Phase != "exhausted" {
			t.Fatal("wheel did not close when prizes exhausted")
		}
		if len(d.Winners) != 3 {
			t.Fatal("wheel stock not conserved")
		}
		expire(c.ID)
		must(s.Draw(ctx, c.ID))
		d, e = s.Detail(ctx, c.ID, "", false)
		must(e)
		if len(d.Winners) != 3 {
			t.Fatal("wheel redrew at end")
		}
	})
	t.Run("entropy_failure_rolls_back", func(t *testing.T) {
		broken := Store{DB: db, Random: failedReader{}}
		c := fixture()
		c.Kind = "wheel"
		c = create(c)
		if _, e = broken.Join(ctx, c.ID, "entropy-user", 10); e == nil {
			t.Fatal("missing entropy accepted")
		}
		d, e := s.Detail(ctx, c.ID, "entropy-user", false)
		must(e)
		var balance, debits int64
		must(db.QueryRow("SELECT balance FROM askxuan_payment.points_account WHERE user_id='entropy-user'").Scan(&balance))
		must(db.QueryRow("SELECT COUNT(*) FROM askxuan_payment.points_ledger WHERE user_id='entropy-user'").Scan(&debits))
		if balance != 1000 || debits != 0 {
			t.Fatal("entropy failure charged points")
		}
		if d.Mine != nil || d.Campaign.ParticipantCount != 0 {
			t.Fatal("partial join persisted")
		}
		c = create(fixture())
		_, e = s.Join(ctx, c.ID, "entropy-1", 10)
		must(e)
		_, e = s.Join(ctx, c.ID, "entropy-2", 10)
		must(e)
		expire(c.ID)
		if e = broken.Draw(ctx, c.ID); e == nil {
			t.Fatal("missing draw entropy accepted")
		}
		d, e = s.Detail(ctx, c.ID, "entropy-1", false)
		must(e)
		if len(d.Winners) != 0 || d.Mine.Outcome != "pending" || d.Campaign.Status != "published" {
			t.Fatal("partial draw persisted")
		}
		must(s.Draw(ctx, c.ID))
	})
	t.Run("draft_private_cancel_and_start", func(t *testing.T) {
		draft, err := s.Save(ctx, fixture(), "admin")
		must(err)
		must(s.Cancel(ctx, draft.ID, "admin", "测试草稿取消"))
		if _, err = s.Detail(ctx, draft.ID, "user", false); !errors.Is(err, ErrNotFound) {
			t.Fatal("unpublished cancellation exposed")
		}

		c := fixture()
		c.StartsAt = time.Now().Unix() + 600
		c.EndsAt += 600
		v, e := s.Save(ctx, c, "admin")
		must(e)
		if _, e = s.Detail(ctx, v.ID, "user", false); !errors.Is(e, ErrNotFound) {
			t.Fatal("draft visible")
		}
		must(s.Publish(ctx, v.ID, "admin"))
		if _, e = s.Join(ctx, v.ID, "user", 10); !errors.Is(e, ErrClosed) {
			t.Fatal("joined before start")
		}
		must(s.Cancel(ctx, v.ID, "admin", "库存准备取消"))
		if _, e = s.Join(ctx, v.ID, "user", 10); !errors.Is(e, ErrClosed) {
			t.Fatal("joined cancelled campaign")
		}
	})
	t.Run("old_binary_cannot_bypass_points", func(t *testing.T) {
		c := create(fixture())
		_, err := db.Exec("INSERT INTO reward_entry(campaign_id,user_id,code,outcome,created_at) VALUES(?,?,'LEGACY-NO-DEBIT','pending',?)", c.ID, "legacy-user", time.Now().Unix())
		if err == nil {
			t.Fatal("old free join bypassed points")
		}
	})
	t.Run("balance_confirmation_and_atomic_failure", func(t *testing.T) {
		c := create(fixture())
		fund("low", 9)
		fund("debt", -2)
		fund("confirmed", 20)
		for _, u := range []string{"low", "missing", "debt"} {
			if _, err := s.Join(ctx, c.ID, u, 10); !errors.Is(err, ErrBalance) {
				t.Fatalf("%s balance: %v", u, err)
			}
		}
		for _, price := range []int64{0, 1, 11} {
			if _, err := s.Join(ctx, c.ID, "confirmed", price); !errors.Is(err, ErrPrice) {
				t.Fatalf("unconfirmed price %d: %v", price, err)
			}
		}
		_, err := db.Exec("CREATE TRIGGER reward_fail_audit BEFORE INSERT ON reward_audit FOR EACH ROW SIGNAL SQLSTATE '45000' SET MESSAGE_TEXT='injected audit failure'")
		must(err)
		_, err = s.Join(ctx, c.ID, "confirmed", 10)
		_, dropErr := db.Exec("DROP TRIGGER reward_fail_audit")
		must(dropErr)
		if err == nil {
			t.Fatal("injected failure ignored")
		}
		d, err := s.Detail(ctx, c.ID, "confirmed", false)
		must(err)
		var n int
		must(db.QueryRow("SELECT COUNT(*) FROM askxuan_payment.points_ledger WHERE user_id='confirmed'").Scan(&n))
		if d.PointsBalance != 20 || d.Mine != nil || d.Campaign.ParticipantCount != 0 || n != 0 {
			t.Fatalf("partial debit/entry: %+v ledger=%d", d, n)
		}
		_, err = s.Join(ctx, c.ID, "confirmed", 10)
		must(err)
	})
	t.Run("concurrent_campaigns_cannot_overspend", func(t *testing.T) {
		first, second := create(fixture()), create(fixture())
		fund("shared-balance", 10)
		errs := make(chan error, 2)
		for _, id := range []int64{first.ID, second.ID} {
			go func(id int64) { _, err := s.Join(ctx, id, "shared-balance", 10); errs <- err }(id)
		}
		ok, low := 0, 0
		for i := 0; i < 2; i++ {
			err := <-errs
			if err == nil {
				ok++
			} else if errors.Is(err, ErrBalance) {
				low++
			} else {
				t.Fatal(err)
			}
		}
		var balance, n int64
		must(db.QueryRow("SELECT balance FROM askxuan_payment.points_account WHERE user_id='shared-balance'").Scan(&balance))
		must(db.QueryRow("SELECT COUNT(*) FROM askxuan_payment.points_ledger WHERE user_id='shared-balance'").Scan(&n))
		if ok != 1 || low != 1 || balance != 0 || n != 1 {
			t.Fatalf("overspend ok=%d low=%d balance=%d ledger=%d", ok, low, balance, n)
		}
	})

	t.Run("production_table_grants_allow_atomic_join_only", func(t *testing.T) {
		_, err := db.Exec("CREATE USER 'marketing_user'@'%' IDENTIFIED BY 'isolated-test-only'")
		must(err)
		_, err = db.Exec("GRANT ALL ON askxuan_rewards_test.* TO 'marketing_user'@'%'")
		must(err)
		permissionSQL, err := os.ReadFile("../../../../../scripts/db/20260910_points_rewards_permissions.sql")
		must(err)
		for _, q := range strings.Split(string(permissionSQL), ";") {
			if strings.TrimSpace(q) != "" {
				_, err = db.Exec(q)
				must(err)
			}
		}
		cfg, err := mysql.ParseDSN(dsn)
		must(err)
		cfg.User = "marketing_user"
		cfg.Passwd = "isolated-test-only"
		limitedDB, err := sql.Open("mysql", cfg.FormatDSN())
		must(err)
		defer limitedDB.Close()
		limited := Store{DB: limitedDB}
		c := create(fixture())
		fund("restricted-user", 30)
		_, err = limited.Join(ctx, c.ID, "restricted-user", 10)
		must(err)
		d, err := limited.Detail(ctx, c.ID, "restricted-user", false)
		must(err)
		if d.PointsBalance != 20 || d.Mine.PointsSpent != 10 {
			t.Fatalf("limited account debit failed: %+v", d)
		}
		if _, err = limitedDB.Exec("DELETE FROM askxuan_payment.points_ledger WHERE user_id='restricted-user'"); err == nil {
			t.Fatal("ledger deletion privilege granted")
		}
	})

}

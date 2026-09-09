package rewards

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	_ "github.com/go-sql-driver/mysql"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func fixture() Campaign {
	now := time.Now().Unix()
	return Campaign{Title: "测试免费好礼", Kind: "pool", PrizeName: "实物礼品", Description: "隔离验收", Rules: "免费参与，平台包邮", PrizeValue: 10000, Budget: 15000, PrizeQuantity: 1, Capacity: 100, StartsAt: now - 10, EndsAt: now + 3600}
}
func TestValidation(t *testing.T) {
	c := fixture()
	if e := Validate(c, time.Now().Unix()); e != nil {
		t.Fatal(e)
	}
	for _, mutate := range []func(*Campaign){func(c *Campaign) { c.Budget = 9999 }, func(c *Campaign) { c.PrizeQuantity = 101 }, func(c *Campaign) { c.EndsAt = c.StartsAt }, func(c *Campaign) { c.Image = "javascript:alert(1)" }, func(c *Campaign) { c.Kind = "points" }, func(c *Campaign) { c.Capacity = 100001 }} {
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
			go func() { defer wg.Done(); v, e := s.Join(ctx, c.ID, "user-0"); out <- v; errs <- e }()
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
			_, e := s.Join(ctx, c.ID, fmt.Sprintf("user-%d", i))
			must(e)
		}
		if _, e := s.Join(ctx, c.ID, "extra"); !errors.Is(e, ErrClosed) {
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
		original, e := s.Join(ctx, c.ID, "user-0")
		must(e)
		if original.ID != id {
			t.Fatal("late retry changed entry")
		}
		if _, e = s.Join(ctx, c.ID, "late-user"); !errors.Is(e, ErrClosed) {
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
				_, e := s.Join(ctx, c.ID, fmt.Sprintf("underfilled-%d", i))
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
			v, e := s.Join(ctx, c.ID, fmt.Sprintf("wheel-%d", i))
			must(e)
			v2, e := s.Join(ctx, c.ID, fmt.Sprintf("wheel-%d", i))
			must(e)
			if v.ID != v2.ID || v.Outcome != v2.Outcome {
				t.Fatal("rerolled wheel")
			}
		}
		d, e := s.Detail(ctx, c.ID, "", false)
		must(e)
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
		if _, e = broken.Join(ctx, c.ID, "entropy-user"); e == nil {
			t.Fatal("missing entropy accepted")
		}
		d, e := s.Detail(ctx, c.ID, "entropy-user", false)
		must(e)
		if d.Mine != nil || d.Campaign.ParticipantCount != 0 {
			t.Fatal("partial join persisted")
		}
		c = create(fixture())
		_, e = s.Join(ctx, c.ID, "entropy-1")
		must(e)
		_, e = s.Join(ctx, c.ID, "entropy-2")
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
		if _, e = s.Join(ctx, v.ID, "user"); !errors.Is(e, ErrClosed) {
			t.Fatal("joined before start")
		}
		must(s.Cancel(ctx, v.ID, "admin", "库存准备取消"))
		if _, e = s.Join(ctx, v.ID, "user"); !errors.Is(e, ErrClosed) {
			t.Fatal("joined cancelled campaign")
		}
	})
}

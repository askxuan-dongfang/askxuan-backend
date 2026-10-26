package logic

import (
	"context"
	"errors"
	"github.com/askxuan/common"
	"sync"
	"testing"
)

func TestReportPriceAmountValidation(t *testing.T) {
	for _, cents := range []int64{-1, 0, MaxReportPriceCents + 1} {
		if _, e := UpdateReportPrice(context.Background(), nil, "1", "dream", ReportPriceUpdate{PriceCents: cents, Revision: 1}); !errors.Is(e, common.ErrParam) {
			t.Fatal("invalid amount accepted", cents, e)
		}
	}
}
func TestMySQLReportPricing(t *testing.T) {
	s := reportTestContext(t)
	ctx := context.Background()
	products, e := ReportPriceProducts(ctx, s.DB)
	if e != nil || len(products) != 12 {
		t.Fatal("pricing catalog", len(products), e)
	}
	for _, p := range products {
		if p.PriceCents != DefaultReportPriceCents || p.Revision != 1 {
			t.Fatal("unexpected default", p)
		}
	}
	updated, e := UpdateReportPrice(ctx, s.DB, "super-fixture", "dream", ReportPriceUpdate{PriceCents: 1490, Revision: 1})
	if e != nil || updated.PriceCents != 1490 || updated.Revision != 2 {
		t.Fatal("update", updated, e)
	}
	var old int64
	if e = s.DB.QueryRowCtx(ctx, &old, `SELECT price_cents FROM ai_report WHERE user_id='9501' AND request_key='mysql-report-case'`); e != nil || old != 990 {
		t.Fatal("existing report repriced", old, e)
	}
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, e := UpdateReportPrice(ctx, s.DB, "super-fixture", "tarot", ReportPriceUpdate{PriceCents: 1800 + int64(i), Revision: 1})
			results <- e
		}(i)
	}
	wg.Wait()
	close(results)
	wins := 0
	for e := range results {
		if e == nil {
			wins++
		} else if !errors.Is(e, ErrReportPriceConflict) {
			t.Fatal(e)
		}
	}
	if wins != 1 {
		t.Fatal("concurrent price writes", wins)
	}
	var audits int
	if e = s.DB.QueryRowCtx(ctx, &audits, `SELECT COUNT(*) FROM ai_report_price_audit`); e != nil || audits != 2 {
		t.Fatal("price audit", audits, e)
	}
}

package cashier

import (
	"context"
	"errors"
	"testing"

	"github.com/askxuan/payment-service/internal/balance"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func checkProviderPendingPayment(t *testing.T, db sqlx.SqlConn) {
	t.Helper()
	ctx := context.Background()
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := db.ExecCtx(ctx, q, args...); err != nil {
			t.Fatal(err)
		}
	}
	exec(`INSERT INTO askxuan_temple.temple VALUES('payment-parent','certified','normal','off_shelf')`)
	exec(`INSERT INTO askxuan_master.master VALUES('payment-master','payment-parent','已认证','normal','off_shelf')`)
	exec(`INSERT INTO askxuan_booking.consultation_order(order_no,user_id,master_code,consult_fee,status,payment_status,payment_no,payment_channel,valid_from,expires_at,valid_hours) VALUES('C-gate-valid','provider-gate','payment-master',9.9,'active','success','fixture','balance',NOW(),DATE_ADD(NOW(),INTERVAL 1 DAY),24),('C-gate','provider-gate','payment-master',9.9,'pending_payment','pending','','',NULL,NULL,24)`)
	exec(`UPDATE askxuan_booking.consultation_order SET temple_code='payment-parent' WHERE order_no IN ('C-gate','C-gate-valid')`)
	exec(`INSERT INTO askxuan_booking.booking(booking_no,user_id,total_fee,status,payment_status,payment_expire_time,slot_reserved,slot_code,master_code,temple_code) VALUES('B-gate','provider-gate',9.9,'pending_payment','pending',DATE_ADD(NOW(),INTERVAL 15 MINUTE),1,'slot','payment-master','payment-parent')`)
	exec(`INSERT INTO askxuan_diy.diy_order(order_no,user_id,total_fee,status,payment_status,bless_fee,pricing_snapshot) VALUES('D-gate','provider-gate',9.9,'pending_review','pending',1,'{"consecration":{"providerType":"temple","templeCode":"payment-parent","masterCode":""}}'),('D-legacy-unknown','provider-gate',9.9,'pending_review','pending',1,NULL)`)
	if err := db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		return balance.Change(ctx, tx, "provider-gate", "provider-gate-fund", "fixture", "test", 10000, 0)
	}); err != nil {
		t.Fatal(err)
	}
	s := Store{DB: db, Enabled: true}
	orders := []OrderRequest{{OrderType: "booking", OrderNo: "B-gate", ExpectedCents: 990, Channel: "balance"}, {OrderType: "consultation", OrderNo: "C-gate", ExpectedCents: 990, Channel: "balance"}, {OrderType: "diy_order", OrderNo: "D-gate", ExpectedCents: 990, Channel: "balance"}}
	reject := func() {
		t.Helper()
		for _, order := range orders {
			if _, err := s.Pay(ctx, "provider-gate", order); !errors.Is(err, balance.ErrConflict) {
				t.Fatalf("blocked provider charge accepted or wrong error %s: %v", order.OrderType, err)
			}
		}
		a, err := balance.Read(ctx, db, "provider-gate")
		if err != nil || a.Available != 10000 {
			t.Fatalf("blocked provider changed wallet: %+v %v", a, err)
		}
		var count int64
		if err := db.QueryRowCtx(ctx, &count, `SELECT COUNT(*) FROM payment WHERE user_id='provider-gate'`); err != nil || count != 0 {
			t.Fatal("blocked provider created payment", err)
		}
	}
	exec(`UPDATE askxuan_temple.temple SET platform_status='banned' WHERE code='payment-parent'`)
	reject()
	// Reassigning a master does not replace the original order's temple snapshot.
	exec(`UPDATE askxuan_master.master SET temple_code='' WHERE code='payment-master'`)
	reject()
	exec(`UPDATE askxuan_master.master SET temple_code='payment-parent' WHERE code='payment-master'`)
	exec(`UPDATE askxuan_temple.temple SET platform_status='normal',auth_status='pending' WHERE code='payment-parent'`)
	reject()
	exec(`UPDATE askxuan_temple.temple SET auth_status='certified' WHERE code='payment-parent'`)
	// A verified off-shelf parent and master still fulfill an already placed order.
	for _, order := range orders {
		paid, err := s.Pay(ctx, "provider-gate", order)
		if err != nil || paid.Amount != 9.9 {
			t.Fatalf("display-only shelf blocked existing price: %s %+v %v", order.OrderType, paid, err)
		}
	}
	before, err := balance.Read(ctx, db, "provider-gate")
	if err != nil || before.Available != 7030 {
		t.Fatalf("fixed snapshot pricing changed: %+v %v", before, err)
	}
	exec(`UPDATE askxuan_temple.temple SET platform_status='banned' WHERE code='payment-parent'`)
	for _, order := range orders {
		if _, err := s.Pay(ctx, "provider-gate", order); err != nil {
			t.Fatal("already paid idempotent read blocked by later ban", err)
		}
	}
	after, err := balance.Read(ctx, db, "provider-gate")
	if err != nil || after.Available != before.Available {
		t.Fatal("paid retry charged after ban", err)
	}
	if _, err := s.Pay(ctx, "provider-gate", OrderRequest{OrderType: "diy_order", OrderNo: "D-legacy-unknown", ExpectedCents: 990, Channel: "balance"}); !errors.Is(err, balance.ErrConflict) {
		t.Fatal("unknown legacy consecration provider charged", err)
	}
}

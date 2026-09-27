package cashier

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/askxuan/common"
	"github.com/askxuan/common/mqoutbox"
	"github.com/askxuan/payment-service/internal/balance"
	"github.com/askxuan/payment-service/internal/model"
	"github.com/askxuan/payment-service/internal/points"
	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"strings"
)

type OrderRequest struct {
	OrderType     string `json:"orderType"`
	OrderNo       string `json:"orderNo"`
	ExpectedCents int64  `json:"expectedCents"`
	Channel       string `json:"channel"`
}
type Quote struct {
	Mode           string         `json:"mode"`
	BalanceChannel string         `json:"balanceChannel"`
	Payment        *model.Payment `json:"payment,omitempty"`
	Cents          int64          `json:"amountCents"`
	Available      int64          `json:"availableCents"`
	BalanceEnabled bool           `json:"balanceEnabled"`
	MockEnabled    bool           `json:"mockEnabled"`
	Experience     bool           `json:"experience"`
}
type querySession interface {
	QueryRowCtx(context.Context, any, string, ...any) error
}
type order struct {
	User    string `db:"user_id"`
	Cents   int64  `db:"cents"`
	Payable int64  `db:"payable"`
}

func orderSQL(kind string) string {
	switch kind {
	case "shop_order":
		return `SELECT user_id,CAST(ROUND(pay_amount*100) AS SIGNED) cents,(status='pending_payment') payable FROM askxuan_order.shop_order WHERE order_no=?`
	case "diy_order":
		return `SELECT user_id,CAST(ROUND(total_fee*100) AS SIGNED) cents,(status='pending_review' AND payment_status='pending') payable FROM askxuan_diy.diy_order WHERE order_no=?`
	case "booking":
		return `SELECT user_id,CAST(ROUND(total_fee*100) AS SIGNED) cents,(status='pending_payment' AND payment_status='pending' AND payment_expire_time>NOW() AND (slot_reserved=1 OR (slot_code='' AND master_code<>''))) payable FROM askxuan_booking.booking WHERE booking_no=?`
	case "consultation":
		return `SELECT user_id,CAST(ROUND(consult_fee*100) AS SIGNED) cents,(status='pending_payment' AND payment_status='pending') payable FROM askxuan_booking.consultation_order WHERE order_no=?`
	case "ai_report":
		return `SELECT user_id,price_cents cents,(status='ready' AND points_paid=0) payable FROM askxuan_ai.ai_report WHERE report_no=?`
	}
	return ""
}
func readOrder(ctx context.Context, q querySession, user string, r OrderRequest, lock bool) (order, error) {
	var o order
	query := orderSQL(r.OrderType)
	if query == "" || r.OrderNo == "" || len(r.OrderNo) > 64 {
		return o, common.ErrParam
	}
	if lock {
		query += " FOR UPDATE"
	}
	e := q.QueryRowCtx(ctx, &o, query, r.OrderNo)
	if errors.Is(e, sqlx.ErrNotFound) {
		return o, common.ErrForbidden
	}
	if e != nil {
		return o, e
	}
	if o.User != user {
		return o, common.ErrForbidden
	}
	if o.Cents <= 0 || o.Cents > balance.MaxCents {
		return o, balance.ErrConflict
	}
	return o, nil
}
func (s *Store) Quote(ctx context.Context, user string, r OrderRequest) (Quote, error) {
	o, e := readOrder(ctx, s.DB, user, r, false)
	if e != nil {
		return Quote{}, e
	}
	if o.Payable != 1 {
		p, e := model.NewPaymentModel(s.DB).FindByIdempotencyKey(ctx, r.OrderType+":"+r.OrderNo)
		if e == nil && p.UserId == user && p.Status == "success" {
			return Quote{Cents: o.Cents, Payment: p}, nil
		}
		return Quote{}, balance.ErrConflict
	}
	a, e := balance.Read(ctx, s.DB, user)
	if s.DemoEnabled {
		a, e = balance.ReadDemo(ctx, s.DB, user)
		return Quote{Mode: "demo", BalanceChannel: "demo_balance", Cents: o.Cents, Available: a.Available, BalanceEnabled: s.Mock, Experience: common.IsExperienceOrder(r.OrderNo)}, e
	}
	experience := common.IsExperienceOrder(r.OrderNo)
	return Quote{Mode: "cash", BalanceChannel: "balance", Cents: o.Cents, Available: a.Available, BalanceEnabled: s.Enabled && !experience, MockEnabled: s.Mock, Experience: experience}, e
}
func (s *Store) Pay(ctx context.Context, user string, r OrderRequest) (*model.Payment, error) {
	if (r.Channel != "balance" || !s.Enabled || s.DemoEnabled) && (r.Channel != "mock" || !s.Mock || s.DemoEnabled) && (r.Channel != "demo_balance" || !s.DemoEnabled || !s.Mock) {
		return nil, balance.ErrUnavailable
	}
	if common.IsExperienceOrder(r.OrderNo) && r.Channel == "balance" {
		return nil, common.NewBizError(40904, "体验订单不能使用真实余额")
	}
	var p model.Payment
	e := s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		// Lock order first, matching AI points and business cancellation lock order.
		o, e := readOrder(ctx, tx, user, r, true)
		if e != nil {
			return e
		}
		e = tx.QueryRowCtx(ctx, &p, `SELECT id,payment_no,COALESCE(idempotency_key,'') idempotency_key,user_id,order_type,order_no,amount,channel,status,trade_no,create_time FROM payment WHERE idempotency_key=?`, r.OrderType+":"+r.OrderNo)
		if e == nil {
			if p.UserId != user || int64(p.Amount*100+0.5) != r.ExpectedCents || p.Channel != r.Channel || p.Status != "success" {
				return balance.ErrConflict
			}
			return nil
		}
		if !errors.Is(e, sqlx.ErrNotFound) {
			return e
		}
		if o.Payable != 1 || o.Cents != r.ExpectedCents {
			return balance.ErrConflict
		}
		no := "P" + strings.ReplaceAll(uuid.NewString(), "-", "")[:30]
		trade := strings.ToUpper(r.Channel) + "-" + no
		res, e := tx.ExecCtx(ctx, `INSERT INTO payment(payment_no,idempotency_key,user_id,order_type,order_no,amount,channel,status,trade_no,create_time) VALUES(?,?,?,?,?,?/100,?,'success',?,NOW())`, no, r.OrderType+":"+r.OrderNo, user, r.OrderType, r.OrderNo, o.Cents, r.Channel, trade)
		if e != nil {
			return e
		}
		id, e := res.LastInsertId()
		if e != nil {
			return e
		}
		if r.Channel == "balance" {
			if e = balance.Change(ctx, tx, user, "pay:"+no, "payment", no, -o.Cents, 0); e != nil {
				return e
			}
		}
		if r.Channel == "demo_balance" {
			if e = balance.ChangeDemo(ctx, tx, user, "pay:"+no, "payment", no, -o.Cents, 0); e != nil {
				return e
			}
		} else if e = points.Award(ctx, tx, id); e != nil {
			return e
		}
		// Commit authoritative business status with cash. No paid-but-cancellable gap.
		switch r.OrderType {
		case "shop_order":
			_, e = tx.ExecCtx(ctx, `UPDATE askxuan_order.shop_order SET status='paid' WHERE order_no=?`, r.OrderNo)
		case "diy_order":
			_, e = tx.ExecCtx(ctx, `UPDATE askxuan_diy.diy_order SET payment_status='success' WHERE order_no=?`, r.OrderNo)
		case "booking":
			_, e = tx.ExecCtx(ctx, `UPDATE askxuan_booking.booking SET status='pending',payment_no=?,payment_channel=?,payment_status='success' WHERE booking_no=?`, no, r.Channel, r.OrderNo)
			if e == nil {
				_, e = tx.ExecCtx(ctx, `INSERT INTO askxuan_booking.booking_status_log(booking_id,from_status,to_status,operator_id,operator_type,remark,create_time) VALUES(?,'pending_payment','pending',?,'user','支付成功，等待确认',NOW())`, r.OrderNo, user)
			}
		case "consultation":
			_, e = tx.ExecCtx(ctx, `UPDATE askxuan_booking.consultation_order SET status='active',payment_no=?,payment_channel=?,payment_status='success',valid_from=NOW(),expires_at=DATE_ADD(NOW(),INTERVAL valid_hours HOUR) WHERE order_no=?`, no, r.Channel, r.OrderNo)
		}
		if e != nil {
			return e
		}
		body, _ := json.Marshal(map[string]any{"paymentNo": no, "userId": user, "orderType": r.OrderType, "orderNo": r.OrderNo, "amount": float64(o.Cents) / 100, "channel": r.Channel, "action": "success"})
		if e = mqoutbox.Enqueue(ctx, tx, "payment:"+no+":success", "payment", no, "payment.success", "payment.events", "", string(body)); e != nil {
			return e
		}
		p = model.Payment{Id: id, PaymentNo: no, UserId: user, OrderType: r.OrderType, OrderNo: r.OrderNo, Amount: float64(o.Cents) / 100, Channel: r.Channel, Status: "success", TradeNo: trade}
		return nil
	})
	return &p, e
}

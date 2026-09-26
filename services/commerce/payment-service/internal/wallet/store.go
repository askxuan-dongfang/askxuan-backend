// Package wallet exposes a read-only projection of payment and refund records.
package wallet

import (
	"context"
	"fmt"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type Store struct{ DB sqlx.SqlConn }
type Summary struct {
	PaidCents      int64 `db:"paid" json:"paidCents"`
	RefundedCents  int64 `db:"refunded" json:"refundedCents"`
	RefundingCents int64 `db:"refunding" json:"refundingCents"`
}
type Refund struct {
	ID          int64  `db:"id" json:"id"`
	PaymentID   int64  `db:"payment_id" json:"-"`
	RefundNo    string `db:"refund_no" json:"refundNo"`
	AmountCents int64  `db:"amount" json:"amountCents"`
	Status      string `db:"status" json:"status"`
	Reason      string `db:"reason" json:"reason"`
	CreatedAt   string `db:"created_at" json:"createdAt"`
}
type Entry struct {
	ID          int64    `db:"id" json:"id"`
	PaymentNo   string   `db:"payment_no" json:"paymentNo"`
	OrderType   string   `db:"order_type" json:"orderType"`
	OrderNo     string   `db:"order_no" json:"orderNo"`
	AmountCents int64    `db:"amount" json:"amountCents"`
	Channel     string   `db:"channel" json:"channel"`
	Status      string   `db:"status" json:"status"`
	CreatedAt   string   `db:"created_at" json:"createdAt"`
	Refunds     []Refund `db:"-" json:"refunds"`
}
type Page struct {
	Summary  Summary `json:"summary"`
	List     []Entry `json:"list"`
	Total    int64   `json:"total"`
	Page     int     `json:"page"`
	PageSize int     `json:"pageSize"`
	Mode     string  `json:"mode"`
}

// Inputs never include a user-selected owner. The handler passes the JWT subject.
func (s Store) Read(ctx context.Context, user, mode, filter string, page int) (Page, error) {
	out := Page{List: []Entry{}, Page: page, PageSize: 20, Mode: mode}
	if user == "" || user == "0" || page < 1 || page > 100000 || (mode != "mock" && mode != "channel") || (filter != "all" && filter != "refunds") {
		return out, fmt.Errorf("invalid wallet query")
	}
	where := "p.user_id=? AND p.channel='mock'"
	if mode == "channel" {
		where = "p.user_id=? AND p.channel<>'mock'"
	}
	// Aggregate each table separately: multiple refunds must not multiply payments.
	err := s.DB.QueryRowCtx(ctx, &out.Summary.PaidCents, `SELECT COALESCE(SUM(CAST(p.amount*100 AS SIGNED)),0) FROM payment p WHERE `+where+` AND p.status IN ('success','refunding','refunded')`, user)
	if err != nil {
		return out, err
	}
	var refunds struct {
		Refunded  int64 `db:"refunded"`
		Refunding int64 `db:"refunding"`
	}
	err = s.DB.QueryRowCtx(ctx, &refunds, `SELECT COALESCE(SUM(IF(r.status='success',CAST(r.amount*100 AS SIGNED),0)),0) refunded,COALESCE(SUM(IF(r.status IN ('pending','processing'),CAST(r.amount*100 AS SIGNED),0)),0) refunding FROM refund r JOIN payment p ON p.id=r.payment_id WHERE `+where, user)
	if err != nil {
		return out, err
	}
	out.Summary.RefundedCents = refunds.Refunded
	out.Summary.RefundingCents = refunds.Refunding
	if filter == "refunds" {
		where += ` AND EXISTS (SELECT 1 FROM refund r WHERE r.payment_id=p.id)`
	}
	if err = s.DB.QueryRowCtx(ctx, &out.Total, `SELECT COUNT(*) FROM payment p WHERE `+where, user); err != nil {
		return out, err
	}
	err = s.DB.QueryRowsPartialCtx(ctx, &out.List, `SELECT p.id,p.payment_no,p.order_type,p.order_no,CAST(p.amount*100 AS SIGNED) amount,p.channel,p.status,DATE_FORMAT(p.create_time,'%Y-%m-%d %H:%i:%s') created_at FROM payment p WHERE `+where+` ORDER BY p.id DESC LIMIT 20 OFFSET ?`, user, (page-1)*20)
	if err != nil {
		return out, err
	}
	for i := range out.List {
		out.List[i].Refunds = []Refund{}
		err = s.DB.QueryRowsCtx(ctx, &out.List[i].Refunds, `SELECT r.id,r.payment_id,r.refund_no,CAST(r.amount*100 AS SIGNED) amount,r.status,r.reason,DATE_FORMAT(r.create_time,'%Y-%m-%d %H:%i:%s') created_at FROM refund r JOIN payment p ON p.id=r.payment_id WHERE p.user_id=? AND p.id=? ORDER BY r.id DESC`, user, out.List[i].ID)
		if err != nil {
			return out, err
		}
	}
	return out, nil
}

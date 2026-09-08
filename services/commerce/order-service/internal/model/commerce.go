package model

import (
	"context"
	"errors"
	"github.com/askxuan/common"
	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"strings"
)

type ReturnDetail struct {
	ReturnOrder
	Carrier    string `db:"carrier" json:"carrier"`
	TrackingNo string `db:"tracking_no" json:"trackingNo"`
	ReviewNote string `db:"review_note" json:"reviewNote"`
}

func ReturnDetails(ctx context.Context, db sqlx.SqlConn, orderID int64) ([]ReturnDetail, error) {
	rows := make([]ReturnDetail, 0)
	err := db.QueryRowsCtx(ctx, &rows, `SELECT r.id,r.return_no,r.order_id,r.type,r.reason,r.status,r.refund_amount,r.create_time,r.update_time,COALESCE(f.carrier,'') carrier,COALESCE(f.tracking_no,'') tracking_no,COALESCE(f.review_note,'') review_note FROM return_order r LEFT JOIN return_fulfillment f ON f.return_id=r.id WHERE r.order_id=? ORDER BY r.id DESC`, orderID)
	return rows, err
}
func CreateReturn(ctx context.Context, db sqlx.SqlConn, user string, id int64, kind, reason string) (ReturnOrder, error) {
	var result ReturnOrder
	if kind != "return" || strings.TrimSpace(reason) == "" || len([]rune(reason)) > 255 {
		return result, common.ErrParam
	}
	err := db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var o struct {
			User   string  `db:"user_id"`
			Status string  `db:"status"`
			Amount float64 `db:"pay_amount"`
		}
		if err := tx.QueryRowCtx(ctx, &o, `SELECT user_id,status,pay_amount FROM shop_order WHERE id=? FOR UPDATE`, id); err != nil {
			return err
		}
		if o.User != user {
			return common.ErrForbidden
		}
		err := tx.QueryRowCtx(ctx, &result, `SELECT id,return_no,order_id,type,reason,status,refund_amount,create_time,update_time FROM return_order WHERE order_id=? AND status!='rejected' ORDER BY id DESC LIMIT 1`, id)
		if err == nil {
			return nil
		}
		if !errors.Is(err, sqlx.ErrNotFound) {
			return err
		}
		if o.Status != "paid" && o.Status != "shipped" && o.Status != "completed" {
			return common.ErrStatusInvalid
		}
		result = ReturnOrder{OrderId: id, ReturnNo: "RO" + strings.ReplaceAll(uuid.NewString(), "-", "")[:30], Type: kind, Reason: strings.TrimSpace(reason), Status: "pending_review", RefundAmount: o.Amount}
		res, err := tx.ExecCtx(ctx, `INSERT INTO return_order(return_no,order_id,type,reason,status,refund_amount,create_time,update_time) VALUES(?,?,?,?,?,?,NOW(),NOW())`, result.ReturnNo, id, kind, result.Reason, result.Status, o.Amount)
		if err != nil {
			return err
		}
		result.Id, err = res.LastInsertId()
		if err != nil {
			return err
		}
		if _, err = tx.ExecCtx(ctx, `INSERT INTO return_fulfillment(return_id,previous_status) VALUES(?,?)`, result.Id, o.Status); err != nil {
			return err
		}
		_, err = tx.ExecCtx(ctx, `UPDATE shop_order SET status='in_return',update_time=NOW() WHERE id=?`, id)
		return err
	})
	return result, err
}
func MoveReturn(ctx context.Context, db sqlx.SqlConn, id int64, user, action, carrier, tracking, note string) error {
	return db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var row struct {
			OrderID  int64  `db:"order_id"`
			Status   string `db:"status"`
			User     string `db:"user_id"`
			Previous string `db:"previous_status"`
			Carrier  string `db:"carrier"`
			Tracking string `db:"tracking_no"`
		}
		err := tx.QueryRowCtx(ctx, &row, `SELECT r.order_id,r.status,o.user_id,COALESCE(f.previous_status,'shipped') previous_status,COALESCE(f.carrier,'') carrier,COALESCE(f.tracking_no,'') tracking_no FROM return_order r JOIN shop_order o ON o.id=r.order_id LEFT JOIN return_fulfillment f ON f.return_id=r.id WHERE r.id=? FOR UPDATE`, id)
		if err != nil {
			return err
		}
		if user != "" && row.User != user {
			return common.ErrForbidden
		}
		target := ""
		switch action {
		case "ship":
			carrier = strings.TrimSpace(carrier)
			tracking = strings.TrimSpace(tracking)
			if carrier == "" || tracking == "" || len([]rune(carrier)) > 80 || len(tracking) > 100 {
				return common.ErrParam
			}
			if row.Status == "return_shipping" && row.Carrier == carrier && row.Tracking == tracking {
				return nil
			}
			if row.Status != "approved" {
				return common.ErrStatusInvalid
			}
			target = "return_shipping"
		case "receive":
			if row.Status == "return_received" {
				return nil
			}
			if row.Status != "return_shipping" {
				return common.ErrStatusInvalid
			}
			target = "return_received"
		case "approve":
			if row.Status != "pending_review" {
				return common.ErrStatusInvalid
			}
			target = "approved"
			if row.Previous == "paid" {
				target = "return_received"
			}
		case "reject":
			if row.Status != "pending_review" {
				return common.ErrStatusInvalid
			}
			if strings.TrimSpace(note) == "" || len([]rune(note)) > 500 {
				return common.ErrParam
			}
			target = "rejected"
		default:
			return common.ErrParam
		}
		if action != "ship" {
			carrier = row.Carrier
			tracking = row.Tracking
		}
		if _, err = tx.ExecCtx(ctx, `INSERT INTO return_fulfillment(return_id,previous_status,carrier,tracking_no,review_note) VALUES(?,?,?,?,?) ON DUPLICATE KEY UPDATE carrier=VALUES(carrier),tracking_no=VALUES(tracking_no),review_note=VALUES(review_note)`, id, row.Previous, carrier, tracking, note); err != nil {
			return err
		}
		if _, err = tx.ExecCtx(ctx, `UPDATE return_order SET status=?,update_time=NOW() WHERE id=?`, target, id); err != nil {
			return err
		}
		if target == "rejected" {
			_, err = tx.ExecCtx(ctx, `UPDATE shop_order SET status=?,update_time=NOW() WHERE id=? AND status='in_return'`, row.Previous, row.OrderID)
		}
		return err
	})
}

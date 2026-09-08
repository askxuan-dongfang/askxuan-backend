package model

import (
	"context"
	"encoding/json"
	"github.com/askxuan/common"
	"github.com/askxuan/common/mqoutbox"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"strings"
)

type OrderLogistics struct {
	ExpressCompany string `json:"expressCompany" db:"express_company"`
	TrackingNo     string `json:"trackingNo" db:"tracking_no"`
	ShipTime       string `json:"shipTime" db:"ship_time"`
}

func ReadOrderLogistics(ctx context.Context, db sqlx.SqlConn, id int64) *OrderLogistics {
	var row OrderLogistics
	if db.QueryRowCtx(ctx, &row, `SELECT express_company,tracking_no,ship_time FROM diy_order_fulfillment WHERE order_id=?`, id) != nil {
		return nil
	}
	return &row
}
func ShipOrder(ctx context.Context, db sqlx.SqlConn, id int64, carrier, tracking string) error {
	carrier = strings.TrimSpace(carrier)
	tracking = strings.TrimSpace(tracking)
	if carrier == "" || tracking == "" || len([]rune(carrier)) > 64 || len(tracking) > 64 {
		return common.ErrParam
	}
	return db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var o struct {
			Status string `db:"status"`
			No     string `db:"order_no"`
			User   string `db:"user_id"`
		}
		if err := tx.QueryRowCtx(ctx, &o, `SELECT status,order_no,user_id FROM diy_order WHERE id=? FOR UPDATE`, id); err != nil {
			return err
		}
		if o.Status == DiyStatusShipped {
			var previous OrderLogistics
			if err := tx.QueryRowCtx(ctx, &previous, `SELECT express_company,tracking_no,ship_time FROM diy_order_fulfillment WHERE order_id=?`, id); err != nil {
				return err
			}
			if previous.ExpressCompany == carrier && previous.TrackingNo == tracking {
				return nil
			}
			return common.ErrStatusInvalid
		}
		if o.Status != DiyStatusAwaitingShipment {
			return common.ErrStatusInvalid
		}
		if _, err := tx.ExecCtx(ctx, `INSERT INTO diy_order_fulfillment(order_id,express_company,tracking_no) VALUES(?,?,?)`, id, carrier, tracking); err != nil {
			return err
		}
		if _, err := tx.ExecCtx(ctx, `UPDATE diy_order SET status='shipped',update_time=NOW() WHERE id=?`, id); err != nil {
			return err
		}
		body, _ := json.Marshal(map[string]interface{}{"orderId": o.No, "orderType": "diy_order", "userId": o.User, "action": "shipped", "expressCompany": carrier, "trackingNo": tracking})
		return mqoutbox.Enqueue(ctx, tx, "diy:order:"+o.No+":shipped", "diy_order", o.No, "order.shipped", "order.events", "", string(body))
	})
}

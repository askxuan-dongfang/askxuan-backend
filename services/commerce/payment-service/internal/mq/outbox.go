package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/askxuan/common/mqoutbox"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func StartOutbox(ctx context.Context, db sqlx.SqlConn, producer *Producer) {
	if db == nil || producer == nil {
		return
	}
	relay := mqoutbox.NewRelay(db, func(ctx context.Context, msg mqoutbox.Message) error {
		return producer.PublishEnvelope(ctx, msg.Exchange, msg.RoutingKey, msg.EventType, msg.EventKey, []byte(msg.Payload))
	})
	go relay.Start(ctx)
	go func() {
		scanPaymentOutbox(ctx, db)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				scanPaymentOutbox(ctx, db)
			}
		}
	}()
}

func scanPaymentOutbox(ctx context.Context, db sqlx.SqlConn) {
	var rows []struct {
		ID        int64   `db:"id"`
		PaymentNo string  `db:"payment_no"`
		UserID    string  `db:"user_id"`
		OrderType string  `db:"order_type"`
		OrderNo   string  `db:"order_no"`
		Amount    float64 `db:"amount"`
		Status    string  `db:"status"`
		EventTime string  `db:"event_time"`
	}
	if err := db.QueryRowsCtx(ctx, &rows, `SELECT p.id,p.payment_no,p.user_id,p.order_type,p.order_no,p.amount,p.status,DATE_FORMAT(p.update_time,'%Y-%m-%d %H:%i:%s') event_time
		FROM payment p LEFT JOIN event_outbox e ON e.event_key=CONCAT('payment:',p.payment_no,':',p.status)
		WHERE p.status IN ('success','failed','refunded') AND e.id IS NULL ORDER BY p.id ASC LIMIT 1000`); err != nil {
		logx.Errorf("payment outbox compensation scan failed: %v", err)
		return
	}
	for _, row := range rows {
		body, _ := json.Marshal(PaymentNotify{PaymentNo: row.PaymentNo, UserId: row.UserID, OrderType: row.OrderType, OrderNo: row.OrderNo, Amount: row.Amount, Action: row.Status, Time: row.EventTime})
		if err := mqoutbox.Enqueue(ctx, db, fmt.Sprintf("payment:%s:%s", row.PaymentNo, row.Status), "payment", row.PaymentNo, "payment."+row.Status, ExchangePaymentEvents, "", string(body)); err != nil {
			logx.Errorf("payment outbox compensation enqueue failed(paymentNo=%s): %v", row.PaymentNo, err)
		}
	}
}

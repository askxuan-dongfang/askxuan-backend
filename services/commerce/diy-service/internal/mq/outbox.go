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
		scanDiyOutbox(ctx, db)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				scanDiyOutbox(ctx, db)
			}
		}
	}()
}

func scanDiyOutbox(ctx context.Context, db sqlx.SqlConn) {
	var tasks []struct {
		ID          int64  `db:"id"`
		TaskNo      string `db:"task_no"`
		OrderNo     string `db:"diy_order_no"`
		TempleCode  string `db:"temple_code"`
		MasterCode  string `db:"master_code"`
		ServiceCode string `db:"service_code"`
	}
	if err := db.QueryRowsCtx(ctx, &tasks, `SELECT t.id,t.task_no,t.diy_order_no,t.temple_code,t.master_code,COALESCE(JSON_UNQUOTE(JSON_EXTRACT(o.design_snapshot,'$.blessServiceCode')),'') service_code
		FROM blessing_task t JOIN diy_order o ON o.order_no=t.diy_order_no
		LEFT JOIN event_outbox e ON e.event_key=CONCAT('diy:blessing:',t.task_no,':dispatch')
		WHERE t.status='dispatched' AND e.id IS NULL ORDER BY t.id ASC LIMIT 1000`); err != nil {
		logx.Errorf("diy outbox blessing scan failed: %v", err)
	} else {
		for _, row := range tasks {
			body, _ := json.Marshal(BlessingDispatch{TaskNo: row.TaskNo, DiyOrderId: row.OrderNo, TempleCode: row.TempleCode, MasterCode: row.MasterCode, ServiceCode: row.ServiceCode})
			if err := mqoutbox.Enqueue(ctx, db, fmt.Sprintf("diy:blessing:%s:dispatch", row.TaskNo), "diy_blessing", row.TaskNo, "blessing.dispatch", ExchangeBlessingEvents, "", string(body)); err != nil {
				logx.Errorf("diy blessing outbox compensation enqueue failed(taskNo=%s): %v", row.TaskNo, err)
			}
		}
	}
	var orders []struct {
		ID      int64  `db:"id"`
		OrderNo string `db:"order_no"`
		UserID  string `db:"user_id"`
	}
	if err := db.QueryRowsCtx(ctx, &orders, `SELECT o.id,o.order_no,o.user_id FROM diy_order o
		LEFT JOIN event_outbox e ON e.event_key=CONCAT('diy:order:',o.order_no,':shipped')
		WHERE o.status='shipped' AND e.id IS NULL ORDER BY o.id ASC LIMIT 1000`); err != nil {
		return
	}
	for _, row := range orders {
		body, _ := json.Marshal(OrderShippedNotify{OrderId: row.OrderNo, UserId: row.UserID, Action: "shipped"})
		if err := mqoutbox.Enqueue(ctx, db, fmt.Sprintf("diy:order:%s:shipped", row.OrderNo), "diy_order", row.OrderNo, "order.shipped", ExchangeOrderEvents, "", string(body)); err != nil {
			logx.Errorf("diy order outbox compensation enqueue failed(orderNo=%s): %v", row.OrderNo, err)
		}
	}
}

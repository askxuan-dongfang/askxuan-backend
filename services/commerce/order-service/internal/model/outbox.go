package model

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// Outbox 状态常量
const (
	OutboxStatusPending = "pending"
	OutboxStatusSent    = "sent"
	OutboxStatusFailed  = "failed"
)

// Outbox 重试上限，超过则标记为 failed
const OutboxMaxRetry = 5

// outbox 表位于 askxuan_order 库（order-service 独占数据库）
const outboxTable = "outbox"

// Outbox 事务性发件箱表，保证业务操作与消息发送的原子性
type Outbox struct {
	Id          int64     `db:"id" json:"id"`
	AggregateId string    `db:"aggregate_id" json:"aggregateId"` // 业务聚合 ID（如退货单号）
	MessageType string    `db:"message_type" json:"messageType"` // 消息类型（如 refund.request）
	Payload     string    `db:"payload" json:"payload"`          // JSON 消息体
	Status      string    `db:"status" json:"status"`            // pending/sent/failed
	RetryCount  int       `db:"retry_count" json:"retryCount"`   // 重试次数
	CreatedAt   time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt   time.Time `db:"updated_at" json:"updatedAt"`
}

// InsertOutbox 同事务插入 outbox 记录（与业务操作在同一事务中）。
// session 既可以是 sqlx.SqlConn（非事务），也可以是 TransactCtx 回调里的 sqlx.Session（事务）。
func InsertOutbox(ctx context.Context, session sqlx.Session, aggregateId, msgType, payload string) error {
	query := fmt.Sprintf(`INSERT INTO %s (aggregate_id, message_type, payload, status, retry_count, created_at, updated_at) VALUES (?, ?, ?, ?, 0, NOW(), NOW())`, outboxTable)
	_, err := session.ExecCtx(ctx, query, aggregateId, msgType, payload, OutboxStatusPending)
	return err
}

// FindPendingOutbox 查询待发送的 outbox 记录（重试次数小于上限）。
// 供 OutboxPublisher 轮询调用。
func FindPendingOutbox(ctx context.Context, conn sqlx.SqlConn, limit int) ([]*Outbox, error) {
	query := fmt.Sprintf(`SELECT id, aggregate_id, message_type, payload, status, retry_count, created_at, updated_at FROM %s WHERE status = ? AND retry_count < ? ORDER BY id LIMIT ?`, outboxTable)
	var list []*Outbox
	if err := conn.QueryRowsCtx(ctx, &list, query, OutboxStatusPending, OutboxMaxRetry, limit); err != nil {
		return nil, err
	}
	return list, nil
}

// MarkOutboxSent 标记 outbox 记录为已发送
func MarkOutboxSent(ctx context.Context, conn sqlx.SqlConn, id int64) error {
	query := fmt.Sprintf(`UPDATE %s SET status = ?, updated_at = NOW() WHERE id = ?`, outboxTable)
	_, err := conn.ExecCtx(ctx, query, OutboxStatusSent, id)
	return err
}

// IncOutboxRetry outbox 发送失败，重试次数 +1；超过上限则标记为 failed
func IncOutboxRetry(ctx context.Context, conn sqlx.SqlConn, id int64) error {
	query := fmt.Sprintf(`UPDATE %s SET retry_count = retry_count + 1, status = CASE WHEN retry_count + 1 >= ? THEN ? ELSE ? END, updated_at = NOW() WHERE id = ?`, outboxTable)
	_, err := conn.ExecCtx(ctx, query, OutboxMaxRetry, OutboxStatusFailed, OutboxStatusPending, id)
	return err
}

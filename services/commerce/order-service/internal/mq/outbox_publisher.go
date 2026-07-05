package mq

import (
	"context"
	"fmt"
	"time"

	"github.com/askxuan/order-service/internal/model"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// OutboxPublisher 轮询 outbox 表，将 pending 状态的消息发送到 MQ。
// 通过事务性发件箱模式保证业务操作与消息发送的原子性：
//  1. 业务逻辑在事务中写 return_order 状态 + outbox 记录
//  2. OutboxPublisher 异步轮询 outbox，将消息发送到 order.events exchange
//  3. 重试上限 OutboxMaxRetry 次，超过则标记为 failed
type OutboxPublisher struct {
	db           sqlx.SqlConn
	producer     *Producer
	pollInterval time.Duration
	batchSize    int
}

// NewOutboxPublisher 构造 OutboxPublisher
func NewOutboxPublisher(db sqlx.SqlConn, producer *Producer) *OutboxPublisher {
	return &OutboxPublisher{
		db:           db,
		producer:     producer,
		pollInterval: 5 * time.Second,
		batchSize:    100,
	}
}

// Start 启动轮询循环，直到 ctx 取消
func (p *OutboxPublisher) Start(ctx context.Context) {
	logx.Info("outbox publisher 启动，轮询间隔 5s")
	ticker := time.NewTicker(p.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logx.Info("outbox publisher 退出")
			return
		case <-ticker.C:
			p.publishPending(ctx)
		}
	}
}

// publishPending 拉取一批 pending 记录并尝试发送
func (p *OutboxPublisher) publishPending(ctx context.Context) {
	list, err := model.FindPendingOutbox(ctx, p.db, p.batchSize)
	if err != nil {
		logx.Errorf("outbox publisher 查询 pending 失败: %v", err)
		return
	}
	if len(list) == 0 {
		return
	}

	for _, o := range list {
		// producer 为 nil（如未配置 MQ）时直接标记 failed，避免无限重试
		if p.producer == nil {
			_ = model.IncOutboxRetry(ctx, p.db, o.Id)
			logx.Errorf("outbox producer 未配置，跳过 id=%d type=%s", o.Id, o.MessageType)
			continue
		}

		messageId := fmt.Sprintf("outbox-%d", o.Id)
		err := p.producer.PublishOutbox(ctx, o.MessageType, messageId, o.Payload)
		if err != nil {
			logx.Errorf("outbox 发送失败 id=%d type=%s: %v", o.Id, o.MessageType, err)
			if err := model.IncOutboxRetry(ctx, p.db, o.Id); err != nil {
				logx.Errorf("outbox 更新重试次数失败 id=%d: %v", o.Id, err)
			}
			continue
		}

		if err := model.MarkOutboxSent(ctx, p.db, o.Id); err != nil {
			logx.Errorf("outbox 标记 sent 失败 id=%d: %v", o.Id, err)
			// 标记失败不影响消息已发送的事实，下次轮询会重复发送（消费端需幂等）
			continue
		}
		logx.Infof("outbox 发送成功 id=%d type=%s aggregate=%s", o.Id, o.MessageType, o.AggregateId)
	}
}

package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/askxuan/common/mqoutbox"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	ExchangePaymentEvents = "payment.events"
)

// PaymentNotify 支付通知事件
type PaymentNotify struct {
	PaymentNo string  `json:"paymentNo"`
	UserId    string  `json:"userId"`
	OrderType string  `json:"orderType"` // booking/shop_order/diy_order
	OrderNo   string  `json:"orderNo"`
	Amount    float64 `json:"amount"`
	Action    string  `json:"action"` // success / failed / refunded
	Time      string  `json:"time"`
}

// RefundCompletedEvent 退款完成事件。
// 比 PaymentNotify(action=refunded) 更结构化，包含 returnNo 便于消费端精确关联退货单。
// 复用 payment.events exchange 发布，order-service 可按需消费。
type RefundCompletedEvent struct {
	ReturnNo  string  `json:"returnNo"`         // 退货单号
	PaymentNo string  `json:"paymentNo"`        // 支付单号
	RefundNo  string  `json:"refundNo"`         // 退款单号
	Status    string  `json:"status"`           // 退款状态: success / failed
	Amount    float64 `json:"amount,omitempty"` // 退款金额
	Time      string  `json:"time"`             // 事件时间
}

// Producer RabbitMQ 生产者
type Producer struct {
	host     string
	port     int
	user     string
	password string
	vhost    string
	db       sqlx.SqlConn

	mu   sync.Mutex
	conn *amqp.Connection
	ch   *amqp.Channel
}

func NewProducer(db sqlx.SqlConn, host string, port int, user, password, vhost string) *Producer {
	return &Producer{
		host: host, port: port, user: user, password: password, vhost: vhost, db: db,
	}
}

func (p *Producer) ensureChannel() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ch != nil && !p.ch.IsClosed() {
		return nil
	}
	if p.conn != nil {
		_ = p.conn.Close()
	}
	url := fmt.Sprintf("amqp://%s:%s@%s:%d%s", p.user, p.password, p.host, p.port, p.vhost)
	conn, err := amqp.Dial(url)
	if err != nil {
		return fmt.Errorf("连接 RabbitMQ 失败: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("打开 channel 失败: %w", err)
	}
	_ = ch.ExchangeDeclare(ExchangePaymentEvents, "fanout", true, false, false, false, nil)
	p.conn = conn
	p.ch = ch
	return nil
}

func (p *Producer) Publish(ctx context.Context, evt PaymentNotify) error {
	if evt.Time == "" {
		evt.Time = time.Now().Format("2006-01-02 15:04:05")
	}
	body, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	return mqoutbox.Enqueue(ctx, p.db, "payment:"+evt.PaymentNo+":"+evt.Action,
		"payment", evt.PaymentNo, "payment."+evt.Action, ExchangePaymentEvents, "", string(body))
}

func (p *Producer) PublishEnvelope(ctx context.Context, exchange, routingKey, eventType, messageID string, payload []byte) error {
	if err := p.ensureChannel(); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.ch.ExchangeDeclare(exchange, "fanout", true, false, false, false, nil); err != nil {
		return err
	}
	return p.ch.PublishWithContext(ctx, exchange, routingKey, false, false, amqp.Publishing{
		ContentType: "application/json", DeliveryMode: amqp.Persistent, Type: eventType,
		MessageId: messageID, Body: payload, Timestamp: time.Now(),
	})
}

// PublishRefundCompleted 发布退款完成事件到 payment.events exchange。
// 消息体为 RefundCompletedEvent（含 returnNo/paymentNo/refundNo/status），
// 消费端可通过 returnNo 精确关联退货单，无需按 orderNo 反查。
func (p *Producer) PublishRefundCompleted(ctx context.Context, evt RefundCompletedEvent) error {
	if evt.Time == "" {
		evt.Time = time.Now().Format("2006-01-02 15:04:05")
	}
	body, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	return mqoutbox.Enqueue(ctx, p.db, "refund:"+evt.RefundNo+":"+evt.Status,
		"refund", evt.RefundNo, "refund.completed", ExchangePaymentEvents, "", string(body))
}

func (p *Producer) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ch != nil {
		_ = p.ch.Close()
	}
	if p.conn != nil {
		_ = p.conn.Close()
	}
}

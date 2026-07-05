package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	ExchangeOrderEvents = "order.events"
)

// OrderNotify 订单状态通知事件
type OrderNotify struct {
	OrderId string `json:"orderId"`
	UserId  string `json:"userId"`
	Action  string `json:"action"` // created / paid / shipped / completed / cancelled / return
	Time    string `json:"time"`
}

// LogisticsSyncEvent 物流同步事件
type LogisticsSyncEvent struct {
	OrderId   string `json:"orderId"`
	OrderType string `json:"orderType"` // shop_order
	ExpressNo string `json:"expressNo"`
	Status    string `json:"status"` // shipped
	Time      string `json:"time"`
}

// Producer RabbitMQ 生产者
type Producer struct {
	host     string
	port     int
	user     string
	password string
	vhost    string

	mu   sync.Mutex
	conn *amqp.Connection
	ch   *amqp.Channel
}

func NewProducer(host string, port int, user, password, vhost string) *Producer {
	return &Producer{
		host: host, port: port, user: user, password: password, vhost: vhost,
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
	_ = ch.ExchangeDeclare(ExchangeOrderEvents, "fanout", true, false, false, false, nil)
	p.conn = conn
	p.ch = ch
	return nil
}

func (p *Producer) Publish(ctx context.Context, evt OrderNotify) error {
	if evt.Time == "" {
		evt.Time = time.Now().Format("2006-01-02 15:04:05")
	}
	body, _ := json.Marshal(evt)
	if err := p.ensureChannel(); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ch.PublishWithContext(ctx, ExchangeOrderEvents, "", false, false,
		amqp.Publishing{ContentType: "application/json", Body: body, DeliveryMode: amqp.Persistent, Timestamp: time.Now()})
}

// PublishOutbox 发布 outbox 消息（用于事务性发件箱模式）。
// 复用 order.events exchange；消息体由调用方提供（payload 已是 JSON 字符串）。
// msgType 写入 amqp.Publishing.Type 字段，消费端可据此路由；messageId 用于幂等去重。
func (p *Producer) PublishOutbox(ctx context.Context, msgType, messageId, payload string) error {
	if err := p.ensureChannel(); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ch.PublishWithContext(ctx, ExchangeOrderEvents, "", false, false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			MessageId:    messageId,
			Type:         msgType,
			Body:         []byte(payload),
			Timestamp:    time.Now(),
		})
}

// PublishLogisticsSync 发布物流同步事件（发货时调用）
func (p *Producer) PublishLogisticsSync(ctx context.Context, evt LogisticsSyncEvent) error {
	if evt.Time == "" {
		evt.Time = time.Now().Format("2006-01-02 15:04:05")
	}
	body, _ := json.Marshal(evt)
	if err := p.ensureChannel(); err != nil {
		return err
	}
	// 声明 logistics 交换机（幂等）
	_ = p.ch.ExchangeDeclare(ExchangeLogisticsEvents, "fanout", true, false, false, false, nil)
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ch.PublishWithContext(ctx, ExchangeLogisticsEvents, "", false, false,
		amqp.Publishing{ContentType: "application/json", Body: body, DeliveryMode: amqp.Persistent, Timestamp: time.Now()})
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

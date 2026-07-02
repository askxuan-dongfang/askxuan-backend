package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// 事件交换机
const (
	ExchangeBlessingEvents = "blessing.events"
	ExchangeOrderEvents    = "order.events"
)

// BlessingDispatch 加持任务派单事件
type BlessingDispatch struct {
	TaskNo      string `json:"taskNo"`
	DiyOrderId  string `json:"diyOrderId"`
	TempleCode  string `json:"templeCode"`
	MasterCode  string `json:"masterCode,omitempty"`
	ServiceCode string `json:"serviceCode"`
	Time        string `json:"time"`
}

// OrderShippedNotify 订单发货通知（与 logistics-service 消费端对齐）
type OrderShippedNotify struct {
	OrderId string `json:"orderId"`
	UserId  string `json:"userId"`
	Action  string `json:"action"` // shipped
	Time    string `json:"time"`
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
	_ = ch.ExchangeDeclare(ExchangeBlessingEvents, "fanout", true, false, false, false, nil)
	_ = ch.ExchangeDeclare(ExchangeOrderEvents, "fanout", true, false, false, false, nil)
	p.conn = conn
	p.ch = ch
	return nil
}

// PublishOrderShipped 发布订单发货事件（供 logistics-service 消费）
func (p *Producer) PublishOrderShipped(ctx context.Context, evt OrderShippedNotify) error {
	if evt.Time == "" {
		evt.Time = time.Now().Format("2006-01-02 15:04:05")
	}
	body, _ := json.Marshal(evt)
	if err := p.ensureChannel(); err != nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ch.PublishWithContext(ctx, ExchangeOrderEvents, "", false, false,
		amqp.Publishing{ContentType: "application/json", Body: body, DeliveryMode: amqp.Persistent, Timestamp: time.Now()})
}

// PublishBlessingDispatch 发布加持派单事件
func (p *Producer) PublishBlessingDispatch(ctx context.Context, evt BlessingDispatch) error {
	if evt.Time == "" {
		evt.Time = time.Now().Format("2006-01-02 15:04:05")
	}
	body, _ := json.Marshal(evt)
	if err := p.ensureChannel(); err != nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ch.PublishWithContext(ctx, ExchangeBlessingEvents, "", false, false,
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

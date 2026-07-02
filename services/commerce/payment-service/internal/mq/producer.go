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
	_ = ch.ExchangeDeclare(ExchangePaymentEvents, "fanout", true, false, false, false, nil)
	p.conn = conn
	p.ch = ch
	return nil
}

func (p *Producer) Publish(ctx context.Context, evt PaymentNotify) error {
	if evt.Time == "" {
		evt.Time = time.Now().Format("2006-01-02 15:04:05")
	}
	body, _ := json.Marshal(evt)
	if err := p.ensureChannel(); err != nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ch.PublishWithContext(ctx, ExchangePaymentEvents, "", false, false,
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

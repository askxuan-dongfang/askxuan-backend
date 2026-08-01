package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// 预约事件交换机约定
const (
	ExchangeBookingEvents = "booking.events" // fanout 交换机
)

// BookingNotify 预约通知事件，由 booking-service 发送、message-service 消费
type BookingNotify struct {
	BookingId     string  `json:"bookingId"`
	UserId        string  `json:"userId"`
	TempleId      string  `json:"templeId"`
	TempleName    string  `json:"templeName,omitempty"`
	MasterId      string  `json:"masterId,omitempty"`
	MasterName    string  `json:"masterName,omitempty"`
	ServiceName   string  `json:"serviceName,omitempty"`
	BookingDate   string  `json:"bookingDate,omitempty"`
	ServiceFee    float64 `json:"serviceFee,omitempty"`
	MeritMoney    float64 `json:"meritMoney,omitempty"`
	TotalFee      float64 `json:"totalFee,omitempty"`
	Rating        int     `json:"rating,omitempty"`
	ReviewContent string  `json:"reviewContent,omitempty"`
	ReviewImages  string  `json:"reviewImages,omitempty"`
	Action        string  `json:"action"` // created / confirmed / completed / reviewed / cancelled
	Time          string  `json:"time"`
}

// Producer RabbitMQ 生产者
// 采用懒连接：首次 Publish 时建立连接，连接断开自动重连
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

// NewProducer 构造生产者（不立即连接，避免 RabbitMQ 未启动时阻塞服务启动）
func NewProducer(host string, port int, user, password, vhost string) *Producer {
	return &Producer{
		host: host, port: port, user: user, password: password, vhost: vhost,
	}
}

// ensureChannel 确保连接与 channel 可用，必要时重连
func (p *Producer) ensureChannel() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.ch != nil && !p.ch.IsClosed() {
		return nil
	}
	// 关闭旧连接
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
	// 声明 fanout 交换机（幂等）
	if err := ch.ExchangeDeclare(ExchangeBookingEvents, "fanout", true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return fmt.Errorf("声明交换机失败: %w", err)
	}
	p.conn = conn
	p.ch = ch
	return nil
}

// Publish 发布预约事件
// 即使 RabbitMQ 不可用也不返回错误，避免影响主流程（仅记录失败）
func (p *Producer) Publish(ctx context.Context, evt BookingNotify) error {
	body, _ := json.Marshal(evt)
	if evt.Time == "" {
		evt.Time = time.Now().Format("2006-01-02 15:04:05")
		body, _ = json.Marshal(evt)
	}
	if err := p.ensureChannel(); err != nil {
		// 连接失败，记录后返回 nil，不阻断主流程
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	err := p.ch.PublishWithContext(ctx,
		ExchangeBookingEvents, // exchange
		"",                    // routing key（fanout 无需）
		false, false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent, // 持久化
			Timestamp:    time.Now(),
		},
	)
	return err
}

// Close 关闭生产者
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

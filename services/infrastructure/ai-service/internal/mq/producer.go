package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// AI 问事事件交换机与队列约定
const (
	ExchangeAIDivination = "ai.events"               // fanout 交换机
	QueueAIDivination    = "ai.divination"           // ai-service 自身消费队列
)

// AIDivination AI 问事事件，由 ai-service 发送并消费（异步调模型）
type AIDivination struct {
	SessionId int64  `json:"sessionId"`
	UserId    string `json:"userId"`
	SkillCode string `json:"skillCode"`
	Content   string `json:"content"` // 用户提问内容
	Time      string `json:"time"`
}

// Producer RabbitMQ 生产者（懒连接，断开自动重连）
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

// NewProducer 构造生产者（不立即连接）
func NewProducer(host string, port int, user, password, vhost string) *Producer {
	return &Producer{
		host: host, port: port, user: user, password: password, vhost: vhost,
	}
}

// ensureChannel 确保连接与 channel 可用
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
	if err := ch.ExchangeDeclare(ExchangeAIDivination, "fanout", true, false, false, false, nil); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return fmt.Errorf("声明交换机失败: %w", err)
	}
	p.conn = conn
	p.ch = ch
	return nil
}

// Publish 发布 AI 问事事件（RabbitMQ 不可用时不阻断主流程）
func (p *Producer) Publish(ctx context.Context, evt AIDivination) error {
	if evt.Time == "" {
		evt.Time = time.Now().Format("2006-01-02 15:04:05")
	}
	body, _ := json.Marshal(evt)
	if err := p.ensureChannel(); err != nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.ch.PublishWithContext(ctx,
		ExchangeAIDivination, "", false, false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
		},
	)
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

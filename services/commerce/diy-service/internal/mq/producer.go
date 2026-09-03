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
	_ = ch.ExchangeDeclare(ExchangeBlessingEvents, "fanout", true, false, false, false, nil)
	_ = ch.ExchangeDeclare(ExchangeOrderEvents, "fanout", true, false, false, false, nil)
	p.conn = conn
	p.ch = ch
	return nil
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

// PublishOrderShipped 发布订单发货事件（供 logistics-service 消费）
func (p *Producer) PublishOrderShipped(ctx context.Context, evt OrderShippedNotify) error {
	if evt.Time == "" {
		evt.Time = time.Now().Format("2006-01-02 15:04:05")
	}
	body, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	return mqoutbox.Enqueue(ctx, p.db, "diy:order:"+evt.OrderId+":"+evt.Action,
		"diy_order", evt.OrderId, "order."+evt.Action, ExchangeOrderEvents, "", string(body))
}

// PublishBlessingDispatch 发布加持派单事件
func (p *Producer) PublishBlessingDispatch(ctx context.Context, evt BlessingDispatch) error {
	if evt.Time == "" {
		evt.Time = time.Now().Format("2006-01-02 15:04:05")
	}
	body, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	return mqoutbox.Enqueue(ctx, p.db, "diy:blessing:"+evt.TaskNo+":dispatch",
		"diy_blessing", evt.TaskNo, "blessing.dispatch", ExchangeBlessingEvents, "", string(body))
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

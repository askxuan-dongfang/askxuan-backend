package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/askxuan/common"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

// 消费队列约定
const (
	QueueDiyBlessingComplete = "diy.blessing.complete"
	QueueDiyPaymentNotify    = "diy.payment.notify"
	QueueDiyLogisticsSync    = "diy.logistics.sync"
)

// 事件交换机（与生产端对齐）
const (
	ExchangePaymentEvents   = "payment.events"
	ExchangeLogisticsEvents = "logistics.events"
)

// BlessingComplete 加持完成回传事件（与 master-service 的 mq.BlessingComplete 对齐）
type BlessingComplete struct {
	EventType  string `json:"eventType"` // "blessing.complete"
	TaskNo     string `json:"taskNo"`
	DiyOrderId string `json:"diyOrderId"`
	TempleCode string `json:"templeCode"`
	MasterCode string `json:"masterCode"`
	Status     string `json:"status"` // completed / failed
	Time       string `json:"time"`
}

// PaymentNotify 支付通知事件（与 payment-service 的 PaymentNotify 对齐）
type PaymentNotify struct {
	PaymentNo string  `json:"paymentNo"`
	UserId    string  `json:"userId"`
	OrderType string  `json:"orderType"` // booking/shop_order/diy_order
	OrderNo   string  `json:"orderNo"`
	Amount    float64 `json:"amount"`
	Action    string  `json:"action"` // success / failed / refunded
	Time      string  `json:"time"`
}

// LogisticsSync 物流状态变更事件（与 logistics-service 的 LogisticsSync 对齐）
type LogisticsSync struct {
	OrderId   string `json:"orderId"`
	OrderType string `json:"orderType"` // shop_order / diy_order
	ExpressNo string `json:"expressNo"`
	Status    string `json:"status"` // shipped / in_transit / delivered / signed
	Time      string `json:"time"`
}

// Binding 队列绑定配置
type Binding struct {
	Exchange string
	Queue    string
	Handler  func([]byte) error
}

// Consumer RabbitMQ 消费者
type Consumer struct {
	host     string
	port     int
	user     string
	password string
	vhost    string
	redis    *redis.Redis
}

// NewConsumer 构造消费者，rds 用于消息幂等性检查
func NewConsumer(host string, port int, user, password, vhost string, rds *redis.Redis) *Consumer {
	return &Consumer{
		host: host, port: port, user: user, password: password, vhost: vhost, redis: rds,
	}
}

// Start 启动消费循环
func (c *Consumer) Start(ctx context.Context, bindings []Binding) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if err := c.consumeLoop(ctx, bindings); err != nil {
				logx.Errorf("diy 消费循环异常，3 秒后重连: %v", err)
				select {
				case <-ctx.Done():
					return
				case <-time.After(3 * time.Second):
				}
			}
		}
	}()
}

// consumeLoop 单次消费循环
func (c *Consumer) consumeLoop(ctx context.Context, bindings []Binding) error {
	url := fmt.Sprintf("amqp://%s:%s@%s:%d%s", c.user, c.password, c.host, c.port, c.vhost)
	conn, err := amqp.Dial(url)
	if err != nil {
		return fmt.Errorf("连接 RabbitMQ 失败: %w", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("打开 channel 失败: %w", err)
	}
	defer ch.Close()

	_ = ch.Qos(1, 0, false)

	type queueConsumer struct {
		msgs     <-chan amqp.Delivery
		handler  func([]byte) error
		name     string
		exchange string
	}
	consumers := make([]queueConsumer, 0, len(bindings))
	for _, b := range bindings {
		_ = ch.ExchangeDeclare(b.Exchange, "fanout", true, false, false, false, nil)
		q, err := ch.QueueDeclare(b.Queue, true, false, false, false, nil)
		if err != nil {
			return fmt.Errorf("声明队列 %s 失败: %w", b.Queue, err)
		}
		if err := ch.QueueBind(q.Name, "", b.Exchange, false, nil); err != nil {
			return fmt.Errorf("绑定队列 %s 到 %s 失败: %w", b.Queue, b.Exchange, err)
		}
		msgs, err := ch.Consume(q.Name, "", false, false, false, false, nil)
		if err != nil {
			return fmt.Errorf("注册消费者 %s 失败: %w", b.Queue, err)
		}
		consumers = append(consumers, queueConsumer{msgs: msgs, handler: b.Handler, name: b.Queue, exchange: b.Exchange})
	}

	closeCh := conn.NotifyClose(make(chan *amqp.Error, 1))
	innerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	for _, qc := range consumers {
		wg.Add(1)
		go func(qc queueConsumer) {
			defer wg.Done()
			for {
				select {
				case <-innerCtx.Done():
					return
				case msg, ok := <-qc.msgs:
					if !ok {
						cancel()
						return
					}
					// 幂等性检查：防止消息重试导致重复处理
					if c.redis != nil {
						messageId := common.ResolveMessageId(msg.MessageId, msg.Body)
						alreadyProcessed, err := common.CheckMessageProcessed(c.redis, qc.exchange, messageId)
						if err != nil {
							logx.Errorf("幂等性检查失败(queue=%s)，nack 重投: %v", qc.name, err)
							_ = msg.Nack(false, true)
							continue
						}
						if alreadyProcessed {
							logx.Infof("消息已处理过(queue=%s messageId=%s)，跳过", qc.name, messageId)
							_ = msg.Ack(false)
							continue
						}
					}
					if err := qc.handler(msg.Body); err != nil {
						logx.Errorf("处理消息失败(queue=%s)，nack 重投: %v", qc.name, err)
						// 回滚幂等标记，允许下次重试
						if c.redis != nil {
							messageId := common.ResolveMessageId(msg.MessageId, msg.Body)
							common.RollbackMessageProcessed(c.redis, qc.exchange, messageId)
						}
						_ = msg.Nack(false, true)
					} else {
						_ = msg.Ack(false)
					}
				}
			}
		}(qc)
	}

	select {
	case <-ctx.Done():
		cancel()
	case err := <-closeCh:
		if err != nil {
			return fmt.Errorf("连接关闭: %w", err)
		}
		return fmt.Errorf("连接已关闭")
	}

	wg.Wait()
	return nil
}

// Close 关闭消费者
func (c *Consumer) Close() {
	// no-op
}

// ParseBlessingComplete 从消息体解析加持完成事件
// 仅处理 eventType == "blessing.complete"
func ParseBlessingComplete(body []byte) (BlessingComplete, bool) {
	var probe struct {
		EventType string `json:"eventType"`
	}
	_ = json.Unmarshal(body, &probe)
	if probe.EventType != "blessing.complete" {
		return BlessingComplete{}, false
	}
	var evt BlessingComplete
	if err := json.Unmarshal(body, &evt); err != nil {
		return BlessingComplete{}, false
	}
	return evt, true
}

// ParsePaymentNotify 从消息体解析支付通知事件
func ParsePaymentNotify(body []byte) (PaymentNotify, bool) {
	var evt PaymentNotify
	if err := json.Unmarshal(body, &evt); err != nil {
		return PaymentNotify{}, false
	}
	return evt, true
}

// ParseLogisticsSync 从消息体解析物流同步事件
func ParseLogisticsSync(body []byte) (LogisticsSync, bool) {
	var evt LogisticsSync
	if err := json.Unmarshal(body, &evt); err != nil {
		return LogisticsSync{}, false
	}
	return evt, true
}

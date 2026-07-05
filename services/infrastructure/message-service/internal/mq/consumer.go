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

// 各上游服务约定的交换机（fanout）
const (
	ExchangeBookingEvents   = "booking.events"
	ExchangeOrderEvents     = "order.events"
	ExchangePaymentEvents   = "payment.events"
	ExchangeBlessingEvents  = "blessing.events"
	ExchangeReviewEvents    = "review.events"
	ExchangeAuditEvents     = "audit.events"
	ExchangeFinanceEvents   = "finance.events"
	ExchangeLogisticsEvents = "logistics.events"
)

// ===== 事件结构体（与各上游服务对齐，独立定义以避免跨模块依赖）=====

// BookingNotify 预约通知事件（booking.notify + booking.status）
type BookingNotify struct {
	BookingId string `json:"bookingId"`
	UserId    string `json:"userId"`
	TempleId  string `json:"templeId"`
	Action    string `json:"action"` // created / confirmed / completed / cancelled
	Time      string `json:"time"`
}

// OrderNotify 商城订单状态通知（order.status）
type OrderNotify struct {
	OrderId string `json:"orderId"`
	UserId  string `json:"userId"`
	Action  string `json:"action"` // created / paid / shipped / completed / cancelled / return
	Time    string `json:"time"`
}

// PaymentNotify 支付通知（payment.notify）
type PaymentNotify struct {
	PaymentNo string  `json:"paymentNo"`
	UserId    string  `json:"userId"`
	OrderType string  `json:"orderType"` // booking/shop_order/diy_order
	OrderNo   string  `json:"orderNo"`
	Amount    float64 `json:"amount"`
	Action    string  `json:"action"` // success / failed / refunded
	Time      string  `json:"time"`
}

// BlessingComplete 加持完成回传（blessing.complete）
type BlessingComplete struct {
	EventType  string `json:"eventType"` // "blessing.complete"
	TaskNo     string `json:"taskNo"`
	DiyOrderId string `json:"diyOrderId"`
	TempleCode string `json:"templeCode"`
	MasterCode string `json:"masterCode"`
	Status     string `json:"status"` // completed / failed
	Time       string `json:"time"`
}

// ReviewNotify 新评价通知（review.notify）
type ReviewNotify struct {
	ReviewId   string `json:"reviewId"`
	UserId     string `json:"userId"`
	TargetType string `json:"targetType"` // temple / master / product / order
	TargetId   string `json:"targetId"`
	Action     string `json:"action"` // created / replied / reported
	Time       string `json:"time"`
}

// AuditResult 审核结果通知（audit.result）
type AuditResult struct {
	AuditId string `json:"auditId"`
	UserId  string `json:"userId"`
	BizType string `json:"bizType"` // review / master / temple
	BizId   string `json:"bizId"`
	Result  string `json:"result"` // approved / rejected
	Time    string `json:"time"`
}

// WithdrawalNotify 提现审核结果通知（withdrawal.notify）
type WithdrawalNotify struct {
	WithdrawalId string  `json:"withdrawalId"`
	UserId       string  `json:"userId"`
	Amount       float64 `json:"amount"`
	Status       string  `json:"status"` // approved / rejected / paid
	Time         string  `json:"time"`
}

// LogisticsSync 物流状态变更事件（logistics.sync，与 logistics-service 对齐）
type LogisticsSync struct {
	OrderId   string `json:"orderId"`
	OrderType string `json:"orderType"` // shop_order / diy_order
	ExpressNo string `json:"expressNo"`
	Status    string `json:"status"` // shipped / in_transit / delivered / signed
	Time      string `json:"time"`
}

// Binding 队列绑定配置
type Binding struct {
	Exchange string               // 交换机名称
	Queue    string               // 队列名称
	Handler  func([]byte) error // 消息处理函数
}

// Consumer RabbitMQ 消费者，支持多交换机/队列绑定
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

// Start 启动消费循环（支持多个绑定）
// handler 返回 error 时 nack 重投；成功 ack
// 连接断开时自动重连并重新消费
func (c *Consumer) Start(ctx context.Context, bindings []Binding) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			if err := c.consumeLoop(ctx, bindings); err != nil {
				logx.Errorf("message 消费循环异常，3 秒后重连: %v", err)
				select {
				case <-ctx.Done():
					return
				case <-time.After(3 * time.Second):
				}
			}
		}
	}()
}

// consumeLoop 单次消费循环，连接断开时返回错误由上层重连
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

	// 声明交换机、队列、绑定，并注册消费者
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

	// 连接关闭通知
	closeCh := conn.NotifyClose(make(chan *amqp.Error, 1))

	// 为每个队列启动一个 goroutine 处理消息
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

	// 等待连接关闭或上下文取消
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

// Close 关闭消费者（连接在 consumeLoop 中通过 defer 管理，此处为接口兼容）
func (c *Consumer) Close() {
	// no-op: 连接由 consumeLoop 管理，context cancel 后自动退出
}

// ===== 事件解析辅助函数 =====

// ParseBlessingComplete 从消息体解析加持完成事件
// 仅处理 eventType == "blessing.complete" 的消息，其他事件（dispatch/assign）跳过
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

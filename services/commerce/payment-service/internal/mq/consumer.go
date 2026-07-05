package mq

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/askxuan/common"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

// 上游服务约定的交换机
const (
	ExchangeOrderEvents = "order.events"
)

// Binding 队列绑定配置。
// EnableDeadLetter=true 时会为该队列声明 DLX（{queue}.dlx）与 DLQ（{queue}.dlq），
// handler 返回 ErrDeadLetter 时消息会被 NACK(requeue=false) 转入死信队列。
//
// fanout exchange 下同一队列会收到 exchange 上所有消息，MsgType 用于按 amqp Publishing.Type
// 字段过滤：非空时仅把匹配的消息投给 Handler，其余直接 ACK 丢弃。空字符串表示不过滤。
type Binding struct {
	Exchange         string
	Queue            string
	MsgType          string
	Handler          func([]byte) error
	EnableDeadLetter bool
}

// ErrDeadLetter handler 返回该 sentinel 表示消息超过重试上限，应转入死信队列。
// 消费循环会以 Nack(requeue=false) 投递到队列绑定的 DLX。
var ErrDeadLetter = errors.New("message exceeded retry limit, dead letter")

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
				logx.Errorf("payment 消费循环异常，3 秒后重连: %v", err)
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
		msgType  string
		name     string
		exchange string
	}
	consumers := make([]queueConsumer, 0, len(bindings))
	for _, b := range bindings {
		_ = ch.ExchangeDeclare(b.Exchange, "fanout", true, false, false, false, nil)

		// 可选：声明 DLX + DLQ，用于 handler 返回 ErrDeadLetter 时收容消息
		var queueArgs amqp.Table
		if b.EnableDeadLetter {
			dlxName := b.Queue + ".dlx"
			dlqName := b.Queue + ".dlq"
			if err := ch.ExchangeDeclare(dlxName, "fanout", true, false, false, false, nil); err != nil {
				return fmt.Errorf("声明 DLX %s 失败: %w", dlxName, err)
			}
			if _, err := ch.QueueDeclare(dlqName, true, false, false, false, nil); err != nil {
				return fmt.Errorf("声明 DLQ %s 失败: %w", dlqName, err)
			}
			if err := ch.QueueBind(dlqName, "", dlxName, false, nil); err != nil {
				return fmt.Errorf("绑定 DLQ %s 到 %s 失败: %w", dlqName, dlxName, err)
			}
			queueArgs = amqp.Table{"x-dead-letter-exchange": dlxName}
		}

		q, err := ch.QueueDeclare(b.Queue, true, false, false, false, queueArgs)
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
		consumers = append(consumers, queueConsumer{msgs: msgs, handler: b.Handler, msgType: b.MsgType, name: b.Queue, exchange: b.Exchange})
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
					// 按 Binding.MsgType 过滤：不匹配的消息直接 ACK 丢弃（fanout exchange 下会收到无关消息）
					if qc.msgType != "" && msg.Type != qc.msgType {
						_ = msg.Ack(false)
						continue
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
						if errors.Is(err, ErrDeadLetter) {
							logx.Errorf("消息超过重试上限，转入死信队列 (queue=%s)", qc.name)
							// 死信场景不回滚幂等标记，避免从 DLQ 重投后再次进入消费循环
							_ = msg.Nack(false, false) // requeue=false → DLX
						} else {
							logx.Errorf("处理消息失败(queue=%s)，nack 重投: %v", qc.name, err)
							// 回滚幂等标记，允许下次重试
							if c.redis != nil {
								messageId := common.ResolveMessageId(msg.MessageId, msg.Body)
								common.RollbackMessageProcessed(c.redis, qc.exchange, messageId)
							}
							_ = msg.Nack(false, true) // requeue=true
						}
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

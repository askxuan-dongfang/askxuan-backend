package mq

import (
	"context"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/zeromicro/go-zero/core/logx"
)

const (
	ExchangePaymentEvents     = "payment.events"
	QueueBookingPaymentNotify = "booking.payment.notify"
)

type PaymentNotify struct {
	PaymentNo string  `json:"paymentNo"`
	UserId    string  `json:"userId"`
	OrderType string  `json:"orderType"`
	OrderNo   string  `json:"orderNo"`
	Amount    float64 `json:"amount"`
	Action    string  `json:"action"`
}

type Consumer struct {
	host, user, password, vhost string
	port                        int
}

func NewConsumer(host string, port int, user, password, vhost string) *Consumer {
	return &Consumer{host: host, port: port, user: user, password: password, vhost: vhost}
}

func (c *Consumer) Start(ctx context.Context, handler func([]byte) error) {
	go func() {
		for ctx.Err() == nil {
			if err := c.consume(ctx, handler); err != nil {
				logx.Errorf("booking payment.notify 消费异常，3 秒后重连: %v", err)
				select {
				case <-ctx.Done():
					return
				case <-time.After(3 * time.Second):
				}
			}
		}
	}()
}

func (c *Consumer) consume(ctx context.Context, handler func([]byte) error) error {
	url := fmt.Sprintf("amqp://%s:%s@%s:%d%s", c.user, c.password, c.host, c.port, c.vhost)
	conn, err := amqp.Dial(url)
	if err != nil {
		return err
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()
	if err := ch.ExchangeDeclare(ExchangePaymentEvents, "fanout", true, false, false, false, nil); err != nil {
		return err
	}
	q, err := ch.QueueDeclare(QueueBookingPaymentNotify, true, false, false, false, nil)
	if err != nil {
		return err
	}
	if err := ch.QueueBind(q.Name, "", ExchangePaymentEvents, false, nil); err != nil {
		return err
	}
	deliveries, err := ch.Consume(q.Name, "", false, false, false, false, nil)
	if err != nil {
		return err
	}
	closeCh := conn.NotifyClose(make(chan *amqp.Error, 1))
	for {
		select {
		case <-ctx.Done():
			return nil
		case closeErr := <-closeCh:
			return closeErr
		case msg, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("payment.notify delivery closed")
			}
			if err := handler(msg.Body); err != nil {
				_ = msg.Nack(false, true)
				continue
			}
			_ = msg.Ack(false)
		}
	}
}

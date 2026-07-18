package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/zeromicro/go-zero/core/logx"
)

const (
	ExchangeBookingEvents  = "booking.events"
	QueueReviewBookingSync = "review.booking.sync"
)

type BookingReviewed struct {
	BookingId     string `json:"bookingId"`
	UserId        string `json:"userId"`
	MasterId      string `json:"masterId"`
	Rating        int    `json:"rating"`
	ReviewContent string `json:"reviewContent"`
	ReviewImages  string `json:"reviewImages"`
	Action        string `json:"action"`
}

type Consumer struct {
	host                  string
	port                  int
	user, password, vhost string
}

func NewConsumer(host string, port int, user, password, vhost string) *Consumer {
	return &Consumer{host: host, port: port, user: user, password: password, vhost: vhost}
}

func (c *Consumer) Start(ctx context.Context, handler func(context.Context, BookingReviewed) error) {
	go func() {
		for ctx.Err() == nil {
			if err := c.consume(ctx, handler); err != nil {
				logx.Errorf("review booking consumer: %v", err)
				select {
				case <-ctx.Done():
					return
				case <-time.After(3 * time.Second):
				}
			}
		}
	}()
}

func (c *Consumer) consume(ctx context.Context, handler func(context.Context, BookingReviewed) error) error {
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
	if err = ch.ExchangeDeclare(ExchangeBookingEvents, "fanout", true, false, false, false, nil); err != nil {
		return err
	}
	queue, err := ch.QueueDeclare(QueueReviewBookingSync, true, false, false, false, nil)
	if err != nil {
		return err
	}
	if err = ch.QueueBind(queue.Name, "", ExchangeBookingEvents, false, nil); err != nil {
		return err
	}
	msgs, err := ch.Consume(queue.Name, "", false, false, false, false, nil)
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case msg, ok := <-msgs:
			if !ok {
				return fmt.Errorf("queue closed")
			}
			var event BookingReviewed
			if err := json.Unmarshal(msg.Body, &event); err != nil {
				_ = msg.Nack(false, false)
				continue
			}
			if event.Action != "reviewed" {
				_ = msg.Ack(false)
				continue
			}
			if event.BookingId == "" || event.UserId == "" || event.MasterId == "" || event.Rating < 1 || event.Rating > 5 {
				_ = msg.Nack(false, false)
				continue
			}
			if err := handler(ctx, event); err != nil {
				_ = msg.Nack(false, true)
				continue
			}
			_ = msg.Ack(false)
		}
	}
}

package mq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/askxuan/common/mqoutbox"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func StartOutbox(ctx context.Context, db sqlx.SqlConn, producer *Producer) {
	if db == nil || producer == nil {
		return
	}
	relay := mqoutbox.NewRelay(db, func(ctx context.Context, msg mqoutbox.Message) error {
		return producer.PublishEnvelope(ctx, msg.Exchange, msg.RoutingKey, msg.EventType, msg.EventKey, []byte(msg.Payload))
	})
	go relay.Start(ctx)
	go func() {
		scanBookingOutbox(ctx, db)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				scanBookingOutbox(ctx, db)
			}
		}
	}()
}

func scanBookingOutbox(ctx context.Context, db sqlx.SqlConn) {
	var logs []struct {
		LogID       int64   `db:"log_id"`
		BookingID   string  `db:"booking_id"`
		UserID      string  `db:"user_id"`
		TempleID    string  `db:"temple_code"`
		TempleName  string  `db:"temple_name"`
		MasterID    string  `db:"master_code"`
		MasterName  string  `db:"master_name"`
		ServiceName string  `db:"service_name"`
		BookingDate string  `db:"booking_date"`
		ServiceFee  float64 `db:"service_fee"`
		MeritMoney  float64 `db:"merit_money"`
		TotalFee    float64 `db:"total_fee"`
		Status      string  `db:"to_status"`
		EventTime   string  `db:"event_time"`
		Rating      int     `db:"rating"`
		Review      string  `db:"review_content"`
		Images      string  `db:"review_images"`
	}
	err := db.QueryRowsCtx(ctx, &logs, `SELECT l.id log_id,l.booking_id,b.user_id,b.temple_code,b.temple_name,b.master_code,b.master_name,b.service_name,DATE_FORMAT(b.booking_date,'%Y-%m-%d') booking_date,b.service_fee,b.merit_money,b.total_fee,l.to_status,DATE_FORMAT(l.create_time,'%Y-%m-%d %H:%i:%s') event_time,
		COALESCE(r.rating,0) rating,COALESCE(r.content,'') review_content,COALESCE(r.images,'') review_images
		FROM booking_status_log l JOIN booking b ON b.booking_no=l.booking_id
		LEFT JOIN booking_review r ON r.booking_id=l.booking_id AND l.to_status='reviewed'
		LEFT JOIN event_outbox e ON e.event_key=CONCAT('booking:',l.booking_id,':',IF(l.to_status='pending','created',l.to_status))
		WHERE l.to_status<>'pending_payment' AND e.id IS NULL ORDER BY l.id ASC LIMIT 1000`)
	if err != nil {
		logx.Errorf("booking outbox compensation scan failed: %v", err)
		return
	}
	for _, row := range logs {
		action := row.Status
		if row.Status == "pending" {
			action = "created"
		}
		body, _ := json.Marshal(BookingNotify{BookingId: row.BookingID, UserId: row.UserID, TempleId: row.TempleID, TempleName: row.TempleName, MasterId: row.MasterID, MasterName: row.MasterName, ServiceName: row.ServiceName, BookingDate: row.BookingDate, ServiceFee: row.ServiceFee, MeritMoney: row.MeritMoney, TotalFee: row.TotalFee, Rating: row.Rating, ReviewContent: row.Review, ReviewImages: row.Images, Action: action, Time: row.EventTime})
		if err := mqoutbox.Enqueue(ctx, db, fmt.Sprintf("booking:%s:%s", row.BookingID, action), "booking", row.BookingID, "booking."+action, ExchangeBookingEvents, "", string(body)); err != nil {
			logx.Errorf("booking outbox compensation enqueue failed(bookingNo=%s): %v", row.BookingID, err)
		}
	}
	var consultations []struct {
		ID         string  `db:"order_no"`
		UserID     string  `db:"user_id"`
		MasterID   string  `db:"master_code"`
		MasterName string  `db:"master_name"`
		TempleID   string  `db:"temple_code"`
		TempleName string  `db:"temple_name"`
		Fee        float64 `db:"consult_fee"`
		EventTime  string  `db:"event_time"`
	}
	if err := db.QueryRowsCtx(ctx, &consultations, `SELECT c.order_no,c.user_id,c.master_code,c.master_name,c.temple_code,c.temple_name,c.consult_fee,DATE_FORMAT(c.valid_from,'%Y-%m-%d %H:%i:%s') event_time
		FROM consultation_order c LEFT JOIN event_outbox e ON e.event_key=CONCAT('consultation:',c.order_no,':paid')
		WHERE c.payment_status='success' AND c.valid_from IS NOT NULL AND e.id IS NULL ORDER BY c.valid_from ASC LIMIT 1000`); err != nil {
		return
	}
	for _, row := range consultations {
		body, _ := json.Marshal(ConsultationNotify{ConsultationId: row.ID, UserId: row.UserID, MasterId: row.MasterID, MasterName: row.MasterName, TempleId: row.TempleID, TempleName: row.TempleName, ConsultFee: row.Fee, Action: "paid", Time: row.EventTime})
		if err := mqoutbox.Enqueue(ctx, db, "consultation:"+row.ID+":paid", "consultation", row.ID, "consultation.paid", ExchangeConsultationEvents, "", string(body)); err != nil {
			logx.Errorf("consultation outbox compensation enqueue failed(orderNo=%s): %v", row.ID, err)
		}
	}
}

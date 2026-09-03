package mqoutbox

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const maxRetries = 12

type Execer interface {
	ExecCtx(context.Context, string, ...interface{}) (sql.Result, error)
}

type Message struct {
	ID         int64  `db:"id"`
	EventKey   string `db:"event_key"`
	EventType  string `db:"event_type"`
	Exchange   string `db:"exchange_name"`
	RoutingKey string `db:"routing_key"`
	Payload    string `db:"payload"`
	RetryCount int    `db:"retry_count"`
}

type Sender func(context.Context, Message) error

func Enqueue(ctx context.Context, exec Execer, eventKey, aggregateType, aggregateID, eventType, exchange, routingKey, payload string) error {
	if eventKey == "" || eventType == "" || exchange == "" || payload == "" {
		return fmt.Errorf("outbox event fields are incomplete")
	}
	_, err := exec.ExecCtx(ctx, `INSERT IGNORE INTO event_outbox
		(event_key,aggregate_type,aggregate_id,event_type,exchange_name,routing_key,payload,status,retry_count,next_retry_at,created_at,updated_at)
		VALUES(?,?,?,?,?,?,CAST(? AS JSON),'pending',0,NOW(),NOW(),NOW())`,
		eventKey, aggregateType, aggregateID, eventType, exchange, routingKey, payload)
	return err
}

type Relay struct {
	db     sqlx.SqlConn
	sender Sender
	poll   time.Duration
	batch  int
}

func NewRelay(db sqlx.SqlConn, sender Sender) *Relay {
	return &Relay{db: db, sender: sender, poll: 2 * time.Second, batch: 100}
}

func (r *Relay) Start(ctx context.Context) {
	r.run(ctx)
	ticker := time.NewTicker(r.poll)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.run(ctx)
		}
	}
}

func (r *Relay) run(ctx context.Context) {
	_, _ = r.db.ExecCtx(ctx, `UPDATE event_outbox SET status='pending',locked_at=NULL,next_retry_at=NOW(),updated_at=NOW() WHERE status='processing' AND locked_at<DATE_SUB(NOW(),INTERVAL 2 MINUTE)`)
	var rows []Message
	if err := r.db.QueryRowsCtx(ctx, &rows, `SELECT id,event_key,event_type,exchange_name,routing_key,CAST(payload AS CHAR) payload,retry_count
		FROM event_outbox WHERE status='pending' AND next_retry_at<=NOW() ORDER BY id LIMIT ?`, r.batch); err != nil {
		logx.Errorf("outbox relay query failed: %v", err)
		return
	}
	for _, msg := range rows {
		res, err := r.db.ExecCtx(ctx, `UPDATE event_outbox SET status='processing',locked_at=NOW(),updated_at=NOW() WHERE id=? AND status='pending'`, msg.ID)
		if err != nil {
			continue
		}
		if n, _ := res.RowsAffected(); n != 1 {
			continue
		}
		if err := r.sender(ctx, msg); err != nil {
			retry := msg.RetryCount + 1
			delay := retry * retry
			status := "pending"
			if retry >= maxRetries {
				status = "dead"
			}
			_, _ = r.db.ExecCtx(ctx, `UPDATE event_outbox SET status=?,retry_count=?,next_retry_at=DATE_ADD(NOW(),INTERVAL ? SECOND),locked_at=NULL,last_error=?,updated_at=NOW() WHERE id=?`, status, retry, delay, truncate(err.Error(), 500), msg.ID)
			continue
		}
		_, _ = r.db.ExecCtx(ctx, `UPDATE event_outbox SET status='sent',sent_at=NOW(),locked_at=NULL,last_error='',updated_at=NOW() WHERE id=?`, msg.ID)
	}
}

func Stats(ctx context.Context, db sqlx.SqlConn) (pending, dead int64, oldestSeconds int64, err error) {
	err = db.QueryRowCtx(ctx, &pending, `SELECT COUNT(*) FROM event_outbox WHERE status IN ('pending','processing')`)
	if err != nil {
		return
	}
	err = db.QueryRowCtx(ctx, &dead, `SELECT COUNT(*) FROM event_outbox WHERE status='dead'`)
	if err != nil {
		return
	}
	err = db.QueryRowCtx(ctx, &oldestSeconds, `SELECT COALESCE(TIMESTAMPDIFF(SECOND,MIN(created_at),NOW()),0) FROM event_outbox WHERE status IN ('pending','processing')`)
	return
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

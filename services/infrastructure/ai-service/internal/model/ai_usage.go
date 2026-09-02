package model

import (
	"context"
	"errors"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var ErrQuotaExceeded = errors.New("ai quota exceeded")

type UsageSummary struct {
	MinuteRequests  int
	DailyRequests   int
	DailyTokens     int64
	DailyCostMicros int64
}

type UsageRecord struct {
	UserID           string
	SessionID        int64
	MessageID        int64
	SkillCode        string
	Provider         string
	Model            string
	PromptTokens     int
	CompletionTokens int
	CostMicros       int64
	Status           string
	LatencyMS        int
	ErrorMessage     string
}

type UsageModel interface {
	Acquire(ctx context.Context, userID string, minuteLimit, dailyLimit int) error
	Record(ctx context.Context, record UsageRecord) error
	Summary(ctx context.Context, userID string) (*UsageSummary, error)
}

type usageModel struct{ conn sqlx.SqlConn }

func NewUsageModel(conn sqlx.SqlConn) UsageModel { return &usageModel{conn: conn} }

func (m *usageModel) Acquire(ctx context.Context, userID string, minuteLimit, dailyLimit int) error {
	now := time.Now()
	minute := now.Truncate(time.Minute)
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return m.conn.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		if err := acquireBucket(ctx, tx, userID, "minute", minute, minuteLimit); err != nil {
			return err
		}
		return acquireBucket(ctx, tx, userID, "day", day, dailyLimit)
	})
}

func acquireBucket(ctx context.Context, tx sqlx.Session, userID, bucketType string, start time.Time, limit int) error {
	if _, err := tx.ExecCtx(ctx, "INSERT IGNORE INTO ai_usage_counter(user_id,bucket_type,bucket_start) VALUES(?,?,?)", userID, bucketType, start); err != nil {
		return err
	}
	var count int
	if err := tx.QueryRowCtx(ctx, &count, "SELECT request_count FROM ai_usage_counter WHERE user_id=? AND bucket_type=? AND bucket_start=? FOR UPDATE", userID, bucketType, start); err != nil {
		return err
	}
	if limit > 0 && count >= limit {
		return ErrQuotaExceeded
	}
	_, err := tx.ExecCtx(ctx, "UPDATE ai_usage_counter SET request_count=request_count+1 WHERE user_id=? AND bucket_type=? AND bucket_start=?", userID, bucketType, start)
	return err
}

func (m *usageModel) Record(ctx context.Context, record UsageRecord) error {
	now := time.Now()
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	totalTokens := int64(record.PromptTokens + record.CompletionTokens)
	if len([]rune(record.ErrorMessage)) > 255 {
		record.ErrorMessage = string([]rune(record.ErrorMessage)[:255])
	}
	return m.conn.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var previous struct {
			TotalTokens int64 `db:"total_tokens"`
			CostMicros  int64 `db:"cost_micros"`
		}
		err := tx.QueryRowCtx(ctx, &previous, "SELECT total_tokens,cost_micros FROM ai_usage_log WHERE message_id=? FOR UPDATE", record.MessageID)
		switch {
		case errors.Is(err, sqlx.ErrNotFound):
			_, err = tx.ExecCtx(ctx, `INSERT INTO ai_usage_log(user_id,session_id,message_id,skill_code,provider,model,prompt_tokens,completion_tokens,total_tokens,cost_micros,status,latency_ms,error_message)
				VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`, record.UserID, record.SessionID, record.MessageID, record.SkillCode, record.Provider, record.Model, record.PromptTokens, record.CompletionTokens, totalTokens, record.CostMicros, record.Status, record.LatencyMS, record.ErrorMessage)
		case err == nil:
			_, err = tx.ExecCtx(ctx, `UPDATE ai_usage_log SET user_id=?,session_id=?,skill_code=?,provider=?,model=?,prompt_tokens=?,completion_tokens=?,total_tokens=?,cost_micros=?,status=?,latency_ms=?,error_message=? WHERE message_id=?`,
				record.UserID, record.SessionID, record.SkillCode, record.Provider, record.Model, record.PromptTokens, record.CompletionTokens, totalTokens, record.CostMicros, record.Status, record.LatencyMS, record.ErrorMessage, record.MessageID)
		default:
			return err
		}
		if err != nil {
			return err
		}
		tokenDelta := totalTokens - previous.TotalTokens
		costDelta := record.CostMicros - previous.CostMicros
		_, err = tx.ExecCtx(ctx, `UPDATE ai_usage_counter SET total_tokens=GREATEST(0,total_tokens+?),cost_micros=GREATEST(0,cost_micros+?)
			WHERE user_id=? AND bucket_type='day' AND bucket_start=?`, tokenDelta, costDelta, record.UserID, day)
		return err
	})
}

func (m *usageModel) Summary(ctx context.Context, userID string) (*UsageSummary, error) {
	now := time.Now()
	minute := now.Truncate(time.Minute)
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	result := &UsageSummary{}
	if err := m.conn.QueryRowCtx(ctx, &result.MinuteRequests, "SELECT COALESCE(MAX(request_count),0) FROM ai_usage_counter WHERE user_id=? AND bucket_type='minute' AND bucket_start=?", userID, minute); err != nil {
		return nil, err
	}
	var row struct {
		Requests int   `db:"request_count"`
		Tokens   int64 `db:"total_tokens"`
		Cost     int64 `db:"cost_micros"`
	}
	if err := m.conn.QueryRowCtx(ctx, &row, "SELECT COALESCE(MAX(request_count),0) request_count,COALESCE(MAX(total_tokens),0) total_tokens,COALESCE(MAX(cost_micros),0) cost_micros FROM ai_usage_counter WHERE user_id=? AND bucket_type='day' AND bucket_start=?", userID, day); err != nil {
		return nil, err
	}
	result.DailyRequests, result.DailyTokens, result.DailyCostMicros = row.Requests, row.Tokens, row.Cost
	return result, nil
}

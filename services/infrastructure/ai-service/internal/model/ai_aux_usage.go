package model

import (
	"context"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"time"
)

func RecordAuxiliaryUsage(ctx context.Context, db sqlx.SqlConn, id string, r UsageRecord) error {
	now := time.Now()
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	return db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		result, e := tx.ExecCtx(ctx, `INSERT IGNORE INTO ai_aux_usage_log(id,user_id,purpose,provider,model,prompt_tokens,completion_tokens,cost_micros,status,latency_ms) VALUES(?,?,?,?,?,?,?,?,?,?)`, id, r.UserID, r.SkillCode, r.Provider, r.Model, r.PromptTokens, r.CompletionTokens, r.CostMicros, r.Status, r.LatencyMS)
		if e != nil {
			return e
		}
		n, e := result.RowsAffected()
		if e != nil || n == 0 {
			return e
		}
		_, e = tx.ExecCtx(ctx, `UPDATE ai_usage_counter SET total_tokens=total_tokens+?,cost_micros=cost_micros+? WHERE user_id=? AND bucket_type='day' AND bucket_start=?`, r.PromptTokens+r.CompletionTokens, r.CostMicros, r.UserID, day)
		return e
	})
}

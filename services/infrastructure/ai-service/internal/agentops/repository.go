package agentops

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type SQLRepository struct {
	DB          sqlx.SqlConn
	RuntimeMode string
}

func (r *SQLRepository) State(ctx context.Context) (State, error) {
	var s State
	err := r.DB.QueryRowCtx(ctx, &s, `SELECT revision,COALESCE(draft_json,'') draft_json,active_version FROM ai_agent_workspace WHERE id=1`)
	return s, err
}
func affected(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrConflict
	}
	return nil
}
func (r *SQLRepository) Save(ctx context.Context, revision int64, definition, actor string) error {
	return r.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		if err := affected(tx.ExecCtx(ctx, `UPDATE ai_agent_workspace SET draft_json=?,revision=revision+1 WHERE id=1 AND revision=?`, definition, revision)); err != nil {
			return err
		}
		_, err := tx.ExecCtx(ctx, `INSERT INTO ai_agent_audit(action,actor,note) VALUES('save',?,'保存草稿')`, actor)
		return err
	})
}
func (r *SQLRepository) Tested(ctx context.Context, revision, providerRevision int64) (bool, error) {
	return tested(ctx, r.DB, revision, providerRevision, r.RuntimeMode)
}
func tested(ctx context.Context, tx sqlx.Session, revision, providerRevision int64, engine ...string) (bool, error) {
	var n int
	mode := ""
	if len(engine) > 0 {
		mode = engine[0]
	}
	err := tx.QueryRowCtx(ctx, &n, `SELECT COUNT(*) FROM ai_agent_evaluation WHERE revision=? AND provider_revision=? AND status='passed' AND create_time>DATE_SUB(NOW(),INTERVAL 1 DAY)`+engineFilter(mode), revision, providerRevision)
	return n > 0, err
}
func (r *SQLRepository) Publish(ctx context.Context, revision, providerRevision int64, actor, note string) (int64, error) {
	var id int64
	err := r.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var s State
		if err := tx.QueryRowCtx(ctx, &s, `SELECT revision,COALESCE(draft_json,'') draft_json,active_version FROM ai_agent_workspace WHERE id=1 FOR UPDATE`); err != nil {
			return err
		}
		if s.Revision != revision {
			return ErrConflict
		}
		if s.Draft == "" {
			return ErrUntested
		}
		ok, err := tested(ctx, tx, revision, providerRevision, r.RuntimeMode)
		if err != nil {
			return err
		}
		if !ok {
			return ErrUntested
		}
		f, err := Decode(s.Draft)
		if err != nil {
			return err
		}
		if err = validateCases(f.Config.Evaluation, f.Config.Skills, true); err != nil {
			return err
		}
		var approval struct {
			ID   string `db:"id"`
			Hash string `db:"suite_hash"`
		}
		if err = tx.QueryRowCtx(ctx, &approval, `SELECT id,suite_hash FROM ai_agent_evaluation WHERE revision=? AND provider_revision=? AND status='passed' AND create_time>DATE_SUB(NOW(),INTERVAL 1 DAY)`+engineFilter(r.RuntimeMode)+` ORDER BY create_time DESC LIMIT 1`, revision, providerRevision); err != nil {
			return err
		}
		f.Approval = &EvaluationApproval{Engine: r.RuntimeMode, ID: approval.ID, ProviderRevision: providerRevision, SuiteHash: approval.Hash}
		raw, _ := json.Marshal(f)
		result, err := tx.ExecCtx(ctx, `INSERT INTO ai_agent_version(definition_json,actor,note) VALUES(?,?,?)`, string(raw), actor, note)
		if err != nil {
			return err
		}
		id, err = result.LastInsertId()
		if err != nil {
			return err
		}
		if err = affected(tx.ExecCtx(ctx, `UPDATE ai_agent_workspace SET active_version=?,revision=revision+1 WHERE id=1 AND revision=?`, id, revision)); err != nil {
			return err
		}
		if err = affected(tx.ExecCtx(ctx, `UPDATE ai_agent_rollout SET stable_version=IF(percentage=100,version_id,stable_version),version_id=?,percentage=0,revision=revision+1 WHERE id=1`, id)); err != nil {
			return err
		}
		_, err = tx.ExecCtx(ctx, `INSERT INTO ai_agent_audit(action,version_id,actor,note) VALUES('publish',?,?,?)`, id, actor, note)
		return err
	})
	return id, err
}
func (r *SQLRepository) Rollback(ctx context.Context, revision, version int64, actor, note string) error {
	return r.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		if version > 0 {
			var id int64
			if err := tx.QueryRowCtx(ctx, &id, `SELECT id FROM ai_agent_version WHERE id=?`, version); err != nil {
				return err
			}
		}
		if err := affected(tx.ExecCtx(ctx, `UPDATE ai_agent_workspace SET active_version=?,revision=revision+1 WHERE id=1 AND revision=?`, version, revision)); err != nil {
			return err
		}
		if err := affected(tx.ExecCtx(ctx, `UPDATE ai_agent_rollout SET version_id=?,stable_version=?,percentage=100,revision=revision+1 WHERE id=1`, version, version)); err != nil {
			return err
		}
		_, err := tx.ExecCtx(ctx, `INSERT INTO ai_agent_audit(action,version_id,actor,note) VALUES('rollback',?,?,?)`, version, actor, note)
		return err
	})
}

const versionRows = `id,definition_json,actor,note,create_time`

func (r *SQLRepository) Version(ctx context.Context, id int64) (Version, error) {
	var v Version
	err := r.DB.QueryRowCtx(ctx, &v, `SELECT `+versionRows+` FROM ai_agent_version WHERE id=?`, id)
	return v, err
}
func (r *SQLRepository) Versions(ctx context.Context) ([]Version, error) {
	v := []Version{}
	err := r.DB.QueryRowsCtx(ctx, &v, `SELECT `+versionRows+` FROM ai_agent_version ORDER BY id DESC LIMIT 50`)
	return v, err
}
func (r *SQLRepository) Audit(ctx context.Context) ([]Audit, error) {
	a := []Audit{}
	err := r.DB.QueryRowsCtx(ctx, &a, `SELECT id,action,version_id,actor,note,create_time FROM ai_agent_audit ORDER BY id DESC LIMIT 50`)
	return a, err
}
func (r *SQLRepository) PutDebug(ctx context.Context, d DebugRun) error {
	// Retain diagnostic content for 30 days; new runs perform bounded cleanup.
	if d.Status == "running" && len(d.Events) == 0 {
		if _, err := r.DB.ExecCtx(ctx, `DELETE FROM ai_agent_checkpoint WHERE expires_at<NOW(3) LIMIT 1000`); err != nil {
			return err
		}
		if _, err := r.DB.ExecCtx(ctx, `DELETE FROM ai_agent_evaluation WHERE create_time<DATE_SUB(NOW(),INTERVAL 30 DAY) LIMIT 1000`); err != nil {
			return err
		}
		if _, err := r.DB.ExecCtx(ctx, `DELETE FROM ai_agent_debug WHERE create_time<DATE_SUB(NOW(),INTERVAL 30 DAY) LIMIT 1000`); err != nil {
			return err
		}
	}
	raw, err := json.Marshal(d)
	if err != nil {
		return err
	}
	_, err = r.DB.ExecCtx(ctx, `INSERT INTO ai_agent_debug(id,actor,revision,provider_revision,kind,status,payload) VALUES(?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE status=VALUES(status),payload=VALUES(payload)`, d.ID, d.Actor, d.Revision, d.ProviderRevision, d.Kind, d.Status, string(raw))
	return err
}
func (r *SQLRepository) Debug(ctx context.Context, id string) (DebugRun, error) {
	var raw string
	var d DebugRun
	err := r.DB.QueryRowCtx(ctx, &raw, `SELECT payload FROM ai_agent_debug WHERE id=? AND create_time>DATE_SUB(NOW(),INTERVAL 30 DAY)`, id)
	if err != nil {
		return d, err
	}
	err = json.Unmarshal([]byte(raw), &d)
	return d, err
}
func (r *SQLRepository) DebugList(ctx context.Context) ([]DebugRun, error) {
	var rows []struct {
		Payload string `db:"payload"`
	}
	out := []DebugRun{}
	if err := r.DB.QueryRowsCtx(ctx, &rows, `SELECT payload FROM ai_agent_debug WHERE create_time>DATE_SUB(NOW(),INTERVAL 30 DAY) ORDER BY create_time DESC LIMIT 50`); err != nil {
		return nil, err
	}
	for _, row := range rows {
		var d DebugRun
		if err := json.Unmarshal([]byte(row.Payload), &d); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, nil
}

type ProductionRun struct {
	ID               int64  `db:"id" json:"id"`
	RunNo            string `db:"run_no" json:"runNo"`
	SkillCode        string `db:"skill_code" json:"skillCode"`
	SkillVersion     string `db:"skill_version" json:"skillVersion"`
	Model            string `db:"model" json:"model"`
	Status           string `db:"status" json:"status"`
	Stage            string `db:"stage" json:"stage"`
	StartedAt        string `db:"started_at" json:"startedAt"`
	LatencyMS        int64  `db:"latency_ms" json:"latencyMs"`
	PromptTokens     int    `db:"prompt_tokens" json:"promptTokens"`
	CompletionTokens int    `db:"completion_tokens" json:"completionTokens"`
	CostMicros       int64  `db:"cost_micros" json:"costMicros"`
}
type ToolTrace struct {
	Name      string `db:"tool_name" json:"name"`
	Status    string `db:"status" json:"status"`
	LatencyMS int    `db:"latency_ms" json:"latencyMs"`
	CreatedAt string `db:"create_time" json:"createdAt"`
}

// No message text, birth data, raw tool result or provider errors in list/detail.
func (r *SQLRepository) ProductionRuns(ctx context.Context, page int, status string) ([]ProductionRun, bool, error) {
	q := `SELECT r.id,r.run_no,r.skill_code,r.skill_version,r.model,r.status,r.stage,r.started_at,COALESCE(TIMESTAMPDIFF(MICROSECOND,r.started_at,r.completed_at) DIV 1000,0) latency_ms,COALESCE(m.prompt_tokens,0) prompt_tokens,COALESCE(m.completion_tokens,0) completion_tokens,COALESCE(m.cost_micros,0) cost_micros FROM ai_run r LEFT JOIN ai_message m ON m.id=r.message_id`
	args := []interface{}{}
	if status != "" {
		q += " WHERE r.status=?"
		args = append(args, status)
	}
	q += " ORDER BY r.id DESC LIMIT 21 OFFSET ?"
	args = append(args, (page-1)*20)
	out := []ProductionRun{}
	err := r.DB.QueryRowsCtx(ctx, &out, q, args...)
	more := len(out) > 20
	if more {
		out = out[:20]
	}
	return out, more, err
}
func (r *SQLRepository) ProductionTools(ctx context.Context, id int64) ([]ToolTrace, error) {
	out := []ToolTrace{}
	err := r.DB.QueryRowsCtx(ctx, &out, `SELECT tool_name,status,latency_ms,create_time FROM ai_tool_call WHERE run_id=? ORDER BY id LIMIT 100`, id)
	return out, err
}

func engineFilter(mode string) string {
	if mode == "harness" {
		return " AND JSON_UNQUOTE(JSON_EXTRACT(payload,'$.engine'))='harness'"
	}
	return ""
}

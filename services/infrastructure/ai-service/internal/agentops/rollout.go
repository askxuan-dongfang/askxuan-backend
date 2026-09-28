package agentops

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"strings"
)

// A stable bucket keeps one account on the same configuration across devices.
type Rollout struct {
	VersionID     int64 `db:"version_id" json:"versionId"`
	StableVersion int64 `db:"stable_version" json:"stableVersion"`
	Percentage    int   `db:"percentage" json:"percentage"`
	Revision      int64 `db:"revision" json:"revision"`
}

func (r Rollout) Select(subject string) int64 {
	if r.Percentage >= 100 {
		return r.VersionID
	}
	if subject != "" && r.Percentage > 0 {
		h := sha256.Sum256([]byte("askxuan-agent-v1:" + subject))
		if int(binary.BigEndian.Uint32(h[:4])%100) < r.Percentage {
			return r.VersionID
		}
	}
	return r.StableVersion
}
func (r *SQLRepository) Rollout(ctx context.Context) (Rollout, error) {
	var v Rollout
	err := r.DB.QueryRowCtx(ctx, &v, `SELECT version_id,stable_version,percentage,revision FROM ai_agent_rollout WHERE id=1`)
	return v, err
}
func (m *Manager) SetRollout(ctx context.Context, revision int64, percentage int, actor, note string) error {
	if percentage < 0 || percentage > 100 || strings.TrimSpace(note) == "" || len([]rune(note)) > 450 {
		return invalid("请填写 0 至 100 的发布比例和 1 至 450 字变更说明")
	}
	if !m.LiveEnabled {
		return invalid("服务端发布开关未开启，请先配置 AI_AGENT_OPERATIONS_ENABLED=true")
	}
	repo, ok := m.Repo.(*SQLRepository)
	if !ok {
		return ErrUnavailable
	}
	_, pr := m.Snapshot()
	return repo.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var state State
		if err := tx.QueryRowCtx(ctx, &state, `SELECT revision,COALESCE(draft_json,'') draft_json,active_version FROM ai_agent_workspace WHERE id=1 FOR UPDATE`); err != nil {
			return err
		}
		var r Rollout
		if err := tx.QueryRowCtx(ctx, &r, `SELECT version_id,stable_version,percentage,revision FROM ai_agent_rollout WHERE id=1 FOR UPDATE`); err != nil {
			return err
		}
		if r.Revision != revision || r.VersionID != state.ActiveVersion {
			return ErrConflict
		}
		if r.VersionID == 0 {
			return invalid("请先发布经过评测的版本")
		}
		if percentage > r.Percentage {
			var raw string
			if err := tx.QueryRowCtx(ctx, &raw, `SELECT definition_json FROM ai_agent_version WHERE id=?`, r.VersionID); err != nil {
				return err
			}
			f, err := Decode(raw)
			if err != nil {
				return err
			}
			if f.Approval == nil || f.Approval.ProviderRevision != pr || (m.RuntimeMode == "harness" && f.Approval.Engine != "harness") {
				return invalid("模型或执行引擎已变化，请重新评测并发布后再扩大范围")
			}
		}
		if err := affected(tx.ExecCtx(ctx, `UPDATE ai_agent_rollout SET percentage=?,revision=revision+1 WHERE id=1 AND revision=?`, percentage, revision)); err != nil {
			return err
		}
		_, err := tx.ExecCtx(ctx, `INSERT INTO ai_agent_audit(action,version_id,actor,note) VALUES('rollout',?,?,?)`, r.VersionID, actor, fmt.Sprintf("覆盖比例 %d%% → %d%%；%s", r.Percentage, percentage, note))
		return err
	})
}

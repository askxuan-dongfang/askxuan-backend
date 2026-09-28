package agentops

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/askxuan/ai-service/internal/agent"
	"github.com/askxuan/ai-service/internal/einopoc"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/cloudwego/eino/components/tool"
	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// Checkpoints are private encrypted envelopes, not part of DebugRun responses.
type checkpointStore struct {
	db     sqlx.SqlConn
	cipher cipher.AEAD
}
type taskSnapshot struct {
	Frozen           Frozen             `json:"frozen"`
	Question         string             `json:"question"`
	Inputs           map[string]any     `json:"inputs"`
	Tool             einopoc.ToolState  `json:"tool"`
	Runner           einopoc.Checkpoint `json:"runner"`
	Created          time.Time          `json:"created"`
	ProviderRevision int64              `json:"providerRevision"`
}
type checkpointRow struct {
	Ciphertext string `db:"ciphertext"`
	Owner      string `db:"owner_token"`
	Valid      int    `db:"valid"`
	Leased     int    `db:"leased"`
}

func (m *Manager) EnablePersistence(key string) error {
	if key == "" {
		return nil
	}
	raw, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(raw) != 32 {
		return errors.New("invalid checkpoint encryption key")
	}
	// Domain separation from provider-settings encryption using the same root key.
	derived := sha256.Sum256(append([]byte("askxuan-agent-checkpoint-v1:"), raw...))
	block, err := aes.NewCipher(derived[:])
	if err != nil {
		return err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	repo, ok := m.Repo.(*SQLRepository)
	if !ok {
		return errors.New("checkpoint SQL repository required")
	}
	m.checkpoints = &checkpointStore{db: repo.DB, cipher: aead}
	return nil
}
func (s *checkpointStore) seal(id string, v taskSnapshot) (string, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	if len(raw) > 3*1024*1024 {
		return "", ErrUnavailable
	}
	nonce := make([]byte, s.cipher.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(s.cipher.Seal(nonce, nonce, raw, []byte(id))), nil
}
func (s *checkpointStore) open(id, encoded string) (taskSnapshot, error) {
	var v taskSnapshot
	data, err := base64.StdEncoding.DecodeString(encoded)
	n := s.cipher.NonceSize()
	if err != nil || len(data) < n || len(data) > 4*1024*1024 {
		return v, ErrUnavailable
	}
	raw, err := s.cipher.Open(nil, data[:n], data[n:], []byte(id))
	if err != nil {
		return v, ErrUnavailable
	}
	err = json.Unmarshal(raw, &v)
	return v, err
}
func (s *checkpointStore) row(ctx context.Context, id string) (checkpointRow, error) {
	var r checkpointRow
	err := s.db.QueryRowCtx(ctx, &r, `SELECT ciphertext,owner_token,expires_at>NOW(3) valid,COALESCE(lease_until>NOW(3),0) leased FROM ai_agent_checkpoint WHERE id=?`, id)
	return r, err
}
func (s *checkpointStore) save(ctx context.Context, t *debugTask, d DebugRun) error {
	c, err := t.harness.Checkpoint()
	if err != nil {
		return err
	}
	v := taskSnapshot{Frozen: t.frozen, Question: t.question, Inputs: t.inputs, Tool: t.bound.State(), Runner: c, Created: t.created, ProviderRevision: d.ProviderRevision}
	encrypted, err := s.seal(d.ID, v)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return s.db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var locked string
		if err := tx.QueryRowCtx(ctx, &locked, `SELECT id FROM ai_agent_debug WHERE id=? FOR UPDATE`, d.ID); err != nil {
			return err
		}
		if t.lease != "" {
			if err := affected(tx.ExecCtx(ctx, `UPDATE ai_agent_checkpoint SET ciphertext=?,owner_token='',lease_until=NULL WHERE id=? AND owner_token=? AND lease_until>NOW(3)`, encrypted, d.ID, t.lease)); err != nil {
				return err
			}
		} else {
			_, err := tx.ExecCtx(ctx, `INSERT INTO ai_agent_checkpoint(id,ciphertext,expires_at) VALUES(?,?,?)`, d.ID, encrypted, t.created.Add(24*time.Hour).UTC().Format("2006-01-02 15:04:05.000"))
			if err != nil {
				return err
			}
		}
		return affected(tx.ExecCtx(ctx, `UPDATE ai_agent_debug SET status=?,payload=? WHERE id=?`, d.Status, string(raw), d.ID))
	})
}

func (m *Manager) resumePersistent(ctx context.Context, actor, id, interrupt string, inputs map[string]any) error {
	raw, err := json.Marshal(inputs)
	if err != nil || len(raw) > 8000 {
		return invalid("补充资料过长")
	}
	d, err := m.Repo.Debug(ctx, id)
	if err != nil {
		return err
	}
	if d.Actor != actor || d.Status != "awaiting_input" || d.InterruptID != interrupt {
		return ErrConflict
	}
	row, err := m.checkpoints.row(ctx, id)
	if err != nil || row.Valid == 0 {
		return ErrExpired
	}
	v, err := m.checkpoints.open(id, row.Ciphertext)
	if err != nil {
		return ErrUnavailable
	}
	snap, revision := m.Snapshot()
	if revision != v.ProviderRevision {
		return invalid("模型连接设置已变更，请结束旧任务并重新调试")
	}
	skill, err := v.Frozen.Skill(d.SkillCode)
	if err != nil {
		return err
	}
	guard := agent.NewGuard(snap.Config.MaxInputChars, snap.Config.BlockedTerms)
	cfg, err := agent.ParseToolConfig(skill.ToolConfig)
	if err != nil {
		return err
	}
	trace := &tracedMCP{base: m.MCP, name: cfg.Tool}
	bound, err := einopoc.NewSkillTool(skill, v.Inputs, v.Question, trace, guard)
	if err != nil {
		return err
	}
	if err = bound.Restore(v.Tool); err != nil {
		return err
	}
	selected, err := snap.Models.Select(ctx, d.Model, false)
	if err != nil {
		return invalid("原任务模型已不可用")
	}
	cap := v.Frozen.Config.MaxOutputTokens
	if cap > 512 {
		cap = 512
	}
	chat, err := provider.NewEinoModel(ctx, snap.Provider, provider.Request{Model: selected, MaxTokens: cap, ThinkingEnabled: snap.Config.ThinkingEnabled, ReasoningEffort: snap.Config.ReasoningEffort})
	if err != nil {
		return err
	}
	h, err := einopoc.NewConfigured(ctx, chat, []tool.BaseTool{bound}, guard, einopoc.Limits{ModelCalls: 4, ToolCalls: 4, Timeout: 45 * time.Second}, v.Frozen.Config.Instruction+"\n"+skill.PromptTemplate)
	if err != nil {
		return err
	}
	if err = h.Restore(v.Runner); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.tasks) >= 8 && m.tasks[id] == nil {
		return invalid("调试容量已满，请稍后再试")
	}
	d, token, err := m.checkpoints.claim(ctx, id, actor, interrupt)
	if err != nil {
		return err
	}
	t := &debugTask{running: true, created: v.Created, run: d, harness: h, bound: bound, lease: token, trace: trace, frozen: v.Frozen, snapshot: snap, question: v.Question, inputs: v.Inputs}
	m.tasks[id] = t
	go m.execute(t, interrupt, string(raw))
	return nil
}

func (m *Manager) inspectPersistent(ctx context.Context, d DebugRun, local bool) (DebugRun, error) {
	r, err := m.checkpoints.row(ctx, d.ID)
	if err == nil {
		if r.Valid != 0 && (d.Status == "awaiting_input" || r.Leased != 0) {
			return d, nil
		}
		prior := d.Status
		d.Status = "expired"
		d.InterruptID = ""
		d.Error = "恢复期限已过或执行被中断，请重新开始"
		b, _ := json.Marshal(d)
		_, err = m.checkpoints.db.ExecCtx(ctx, `UPDATE ai_agent_debug d JOIN ai_agent_checkpoint c ON c.id=d.id SET d.status='expired',d.payload=? WHERE d.id=? AND d.status=? AND (c.expires_at<=NOW(3) OR (d.status='running' AND c.lease_until<=NOW(3)))`, string(b), d.ID, prior)
		if err != nil {
			return d, err
		}
		return m.Repo.Debug(ctx, d.ID)
	}
	if !errors.Is(err, sqlx.ErrNotFound) {
		return d, ErrUnavailable
	}
	started, _ := time.Parse(time.RFC3339Nano, d.StartedAt)
	if local || (d.Status == "running" && time.Since(started) < 70*time.Second) {
		return d, nil
	}
	prior := d.Status
	d.Status = "expired"
	d.InterruptID = ""
	d.Error = "执行中断且没有可恢复检查点，请重新开始"
	b, _ := json.Marshal(d)
	// A checkpoint may have been created after the read; do not expire it.
	_, err = m.checkpoints.db.ExecCtx(ctx, `UPDATE ai_agent_debug d LEFT JOIN ai_agent_checkpoint c ON c.id=d.id SET d.status='expired',d.payload=? WHERE d.id=? AND d.status=? AND c.id IS NULL`, string(b), d.ID, prior)
	if err != nil {
		return d, err
	}
	return m.Repo.Debug(ctx, d.ID)
}

// One pending checkpoint can be claimed once, even across multiple instances.
func (s *checkpointStore) claim(ctx context.Context, id, actor, interrupt string) (DebugRun, string, error) {
	var d DebugRun
	token := uuid.NewString()
	err := s.db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var raw string
		if err := tx.QueryRowCtx(ctx, &raw, `SELECT payload FROM ai_agent_debug WHERE id=? FOR UPDATE`, id); err != nil {
			return err
		}
		if json.Unmarshal([]byte(raw), &d) != nil {
			return ErrUnavailable
		}
		if d.Actor != actor || d.Status != "awaiting_input" || interrupt == "" || d.InterruptID != interrupt {
			return ErrConflict
		}
		if err := affected(tx.ExecCtx(ctx, `UPDATE ai_agent_checkpoint SET owner_token=?,lease_until=DATE_ADD(NOW(3),INTERVAL 70 SECOND) WHERE id=? AND expires_at>NOW(3) AND owner_token=''`, token, id)); err != nil {
			return err
		}
		d.Status = "running"
		b, _ := json.Marshal(d)
		return affected(tx.ExecCtx(ctx, `UPDATE ai_agent_debug SET status='running',payload=? WHERE id=?`, string(b), id))
	})
	return d, token, err
}
func (s *checkpointStore) progress(ctx context.Context, d DebugRun, token string) error {
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return affected(s.db.ExecCtx(ctx, `UPDATE ai_agent_debug d JOIN ai_agent_checkpoint c ON c.id=d.id SET d.status=?,d.payload=? WHERE d.id=? AND c.owner_token=? AND c.lease_until>NOW(3)`, d.Status, string(b), d.ID, token))
}
func (s *checkpointStore) finish(ctx context.Context, d DebugRun, token string) error {
	return s.db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var id string
		if err := tx.QueryRowCtx(ctx, &id, `SELECT id FROM ai_agent_debug WHERE id=? FOR UPDATE`, d.ID); err != nil {
			return err
		}
		if err := affected(tx.ExecCtx(ctx, `DELETE FROM ai_agent_checkpoint WHERE id=? AND owner_token=? AND lease_until>NOW(3)`, d.ID, token)); err != nil {
			return err
		}
		raw, _ := json.Marshal(d)
		_, err := tx.ExecCtx(ctx, `UPDATE ai_agent_debug SET status=?,payload=? WHERE id=?`, d.Status, string(raw), d.ID)
		return err
	})
}
func (s *checkpointStore) cancel(ctx context.Context, id, actor string) error {
	return s.db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var raw string
		if err := tx.QueryRowCtx(ctx, &raw, `SELECT payload FROM ai_agent_debug WHERE id=? FOR UPDATE`, id); err != nil {
			return err
		}
		var d DebugRun
		if json.Unmarshal([]byte(raw), &d) != nil {
			return ErrUnavailable
		}
		if d.Actor != actor || d.Status != "awaiting_input" {
			return ErrConflict
		}
		d.Status = "cancelled"
		d.InterruptID = ""
		b, _ := json.Marshal(d)
		if err := affected(tx.ExecCtx(ctx, `UPDATE ai_agent_debug SET status='cancelled',payload=? WHERE id=?`, string(b), id)); err != nil {
			return err
		}
		_, err := tx.ExecCtx(ctx, `DELETE FROM ai_agent_checkpoint WHERE id=?`, id)
		return err
	})
}

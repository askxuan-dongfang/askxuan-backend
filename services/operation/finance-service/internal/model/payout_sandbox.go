package model

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	SandboxPendingReview = "pending_review"
	SandboxApproved      = "approved"
	SandboxRejected      = "rejected"
	SandboxSuccess       = "success"
	SandboxFailed        = "failed"
	SandboxUnknown       = "unknown"
)

var (
	ErrSandboxConflict = errors.New("sandbox request/version conflict")
	ErrSandboxState    = errors.New("sandbox action is not allowed in this state")
)

type PayoutSandbox struct {
	ID              string `db:"id" json:"id"`
	CreateRequestID string `db:"create_request_id" json:"-"`
	AmountCents     int64  `db:"amount_cents" json:"amountCents"`
	Status          string `db:"status" json:"status"`
	Version         int64  `db:"version" json:"version"`
	Attempt         int    `db:"attempt" json:"attempt"`
	CreatedBy       string `db:"created_by" json:"createdBy"`
	CreatedAt       string `db:"create_time" json:"createdAt"`
	UpdatedAt       string `db:"update_time" json:"updatedAt"`
}

type PayoutSandboxEvent struct {
	ID         int64  `db:"id" json:"id"`
	RequestID  string `db:"request_id" json:"requestId"`
	Stage      string `db:"stage" json:"stage"`
	Action     string `db:"action" json:"action"`
	FromStatus string `db:"from_status" json:"fromStatus"`
	ToStatus   string `db:"to_status" json:"toStatus"`
	Outcome    string `db:"outcome" json:"outcome"`
	Version    int64  `db:"version" json:"version"`
	Attempt    int    `db:"attempt" json:"attempt"`
	ActorID    string `db:"actor_id" json:"actorId"`
	CreatedAt  string `db:"create_time" json:"createdAt"`
}

type PayoutSandboxAction struct {
	RequestID, Action, Outcome, ActorID string
	Version                             int64
}
type PayoutSandboxRepository interface {
	List(context.Context, string, int, int) ([]*PayoutSandbox, int64, error)
	Get(context.Context, string) (*PayoutSandbox, []*PayoutSandboxEvent, error)
	Create(context.Context, string, int64, string) (*PayoutSandbox, error)
	Act(context.Context, string, PayoutSandboxAction) (*PayoutSandbox, error)
}
type payoutSandboxModel struct{ conn sqlx.SqlConn }

func NewPayoutSandboxModel(conn sqlx.SqlConn) PayoutSandboxRepository {
	return &payoutSandboxModel{conn: conn}
}
func DefaultPayoutSandboxModel() PayoutSandboxRepository { return NewPayoutSandboxModel(db) }

const sandboxSelect = `id,create_request_id,amount_cents,status,version,attempt,created_by,DATE_FORMAT(create_time,'%Y-%m-%d %H:%i:%s.%f') create_time,DATE_FORMAT(update_time,'%Y-%m-%d %H:%i:%s.%f') update_time`

func (m *payoutSandboxModel) List(ctx context.Context, status string, page, size int) ([]*PayoutSandbox, int64, error) {
	where, args := "1=1", []any{}
	if status != "" {
		where = "status=?"
		args = append(args, status)
	}
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, "SELECT COUNT(1) FROM payout_sandbox WHERE "+where, args...); err != nil {
		return nil, 0, err
	}
	rows := []*PayoutSandbox{}
	if total == 0 {
		return rows, 0, nil
	}
	args = append(args, (page-1)*size, size)
	err := m.conn.QueryRowsCtx(ctx, &rows, "SELECT "+sandboxSelect+" FROM payout_sandbox WHERE "+where+" ORDER BY create_time DESC,id DESC LIMIT ?,?", args...)
	return rows, total, err
}
func (m *payoutSandboxModel) Get(ctx context.Context, id string) (*PayoutSandbox, []*PayoutSandboxEvent, error) {
	var row PayoutSandbox
	if err := m.conn.QueryRowCtx(ctx, &row, "SELECT "+sandboxSelect+" FROM payout_sandbox WHERE id=?", id); err != nil {
		return nil, nil, err
	}
	events := []*PayoutSandboxEvent{}
	err := m.conn.QueryRowsCtx(ctx, &events, `SELECT id,request_id,stage,action,from_status,to_status,outcome,version,attempt,actor_id,DATE_FORMAT(create_time,'%Y-%m-%d %H:%i:%s.%f') create_time FROM payout_sandbox_event WHERE sandbox_id=? ORDER BY id`, id)
	return &row, events, err
}

func sandboxHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
func insertSandboxEvent(ctx context.Context, tx sqlx.Session, id string, a PayoutSandboxAction, stage, from, to string, version int64, attempt int) error {
	hash := sandboxHash(fmt.Sprintf("%s|%s|%d|%s", a.Action, a.Outcome, a.Version, a.ActorID))
	outcome := a.Outcome
	if stage == "processing" {
		outcome = ""
	}
	_, err := tx.ExecCtx(ctx, `INSERT INTO payout_sandbox_event(sandbox_id,request_id,request_hash,stage,action,from_status,to_status,outcome,version,attempt,actor_id) VALUES(?,?,?,?,?,?,?,?,?,?,?)`, id, a.RequestID, hash, stage, a.Action, from, to, outcome, version, attempt, a.ActorID)
	return err
}

func (m *payoutSandboxModel) Create(ctx context.Context, requestID string, amount int64, actor string) (*PayoutSandbox, error) {
	var row PayoutSandbox
	err := m.conn.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		id := "sandbox_" + uuid.NewString()
		// A competing retry waits for the first transaction; no second instance
		// or event is created. A reused key with changed data is rejected below.
		if _, err := tx.ExecCtx(ctx, `INSERT INTO payout_sandbox(id,create_request_id,amount_cents,created_by) VALUES(?,?,?,?) ON DUPLICATE KEY UPDATE create_request_id=create_request_id`, id, requestID, amount, actor); err != nil {
			return err
		}
		if err := tx.QueryRowCtx(ctx, &row, "SELECT "+sandboxSelect+" FROM payout_sandbox WHERE create_request_id=? FOR UPDATE", requestID); err != nil {
			return err
		}
		if row.AmountCents != amount || row.CreatedBy != actor {
			return ErrSandboxConflict
		}
		var count int
		if err := tx.QueryRowCtx(ctx, &count, `SELECT COUNT(1) FROM payout_sandbox_event WHERE sandbox_id=? AND request_id=?`, row.ID, requestID); err != nil {
			return err
		}
		if count > 0 {
			return nil
		}
		return insertSandboxEvent(ctx, tx, row.ID, PayoutSandboxAction{RequestID: requestID, Action: "create", ActorID: actor}, "created", "", SandboxPendingReview, 1, 0)
	})
	return &row, err
}

func sandboxNext(status, action, outcome string) (string, int64, int, error) {
	switch action {
	case "approve":
		if status == SandboxPendingReview && outcome == "" {
			return SandboxApproved, 1, 0, nil
		}
	case "reject":
		if status == SandboxPendingReview && outcome == "" {
			return SandboxRejected, 1, 0, nil
		}
	case "submit", "retry":
		if (action == "submit" && status == SandboxApproved) || (action == "retry" && status == SandboxFailed) {
			if outcome == SandboxSuccess || outcome == SandboxFailed || outcome == SandboxUnknown {
				return outcome, 2, 1, nil
			}
		}
	case "resolve":
		if status == SandboxUnknown && (outcome == SandboxSuccess || outcome == SandboxFailed) {
			return outcome, 1, 0, nil
		}
	}
	return "", 0, 0, ErrSandboxState
}

func (m *payoutSandboxModel) Act(ctx context.Context, id string, a PayoutSandboxAction) (*PayoutSandbox, error) {
	var row PayoutSandbox
	err := m.conn.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		if err := tx.QueryRowCtx(ctx, &row, "SELECT "+sandboxSelect+" FROM payout_sandbox WHERE id=? FOR UPDATE", id); err != nil {
			return err
		}
		var previous string
		err := tx.QueryRowCtx(ctx, &previous, `SELECT request_hash FROM payout_sandbox_event WHERE sandbox_id=? AND request_id=? ORDER BY id LIMIT 1`, id, a.RequestID)
		if err == nil {
			if previous != sandboxHash(fmt.Sprintf("%s|%s|%d|%s", a.Action, a.Outcome, a.Version, a.ActorID)) {
				return ErrSandboxConflict
			}
			return nil
		}
		if !errors.Is(err, sqlx.ErrNotFound) {
			return err
		}
		if row.Version != a.Version {
			return ErrSandboxConflict
		}
		next, increment, attempt, err := sandboxNext(row.Status, a.Action, a.Outcome)
		if err != nil {
			return err
		}
		from, version := row.Status, row.Version
		result, err := tx.ExecCtx(ctx, `UPDATE payout_sandbox SET status=?,version=version+?,attempt=attempt+? WHERE id=? AND version=?`, next, increment, attempt, id, a.Version)
		if err != nil {
			return err
		}
		changed, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if changed != 1 {
			return ErrSandboxConflict
		}
		if attempt == 1 {
			if err = insertSandboxEvent(ctx, tx, id, a, "processing", from, "processing", version+1, row.Attempt+1); err != nil {
				return err
			}
			if err = insertSandboxEvent(ctx, tx, id, a, "result", "processing", next, version+2, row.Attempt+1); err != nil {
				return err
			}
		} else {
			stage := "decision"
			if a.Action == "resolve" {
				stage = "query_result"
			}
			if err = insertSandboxEvent(ctx, tx, id, a, stage, from, next, version+1, row.Attempt); err != nil {
				return err
			}
		}
		return tx.QueryRowCtx(ctx, &row, "SELECT "+sandboxSelect+" FROM payout_sandbox WHERE id=?", id)
	})
	return &row, err
}

package model

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	SessionStatusActive = "active"
	SessionStatusClosed = "closed"
)

type AISession struct {
	Id            int64  `db:"id"`
	SessionNo     string `db:"session_no"`
	UserId        string `db:"user_id"`
	SkillCode     string `db:"skill_code"`
	SelectionMode string `db:"selection_mode"`
	SkillVersion  string `db:"skill_version"`
	Title         string `db:"title"`
	Status        string `db:"status"`
	CreatedAt     string `db:"create_time"`
	UpdatedAt     string `db:"update_time"`
}

type ConversationModel interface {
	CreateSession(ctx context.Context, userId, skillCode, selectionMode, skillVersion, question, inputJSON, attachmentsJSON string) (*AISession, int64, error)
	ListSessions(ctx context.Context, userId, status string, page, size int) ([]*AISession, int64, error)
	FindSession(ctx context.Context, id int64) (*AISession, error)
	CloseSession(ctx context.Context, id int64, userId string) (bool, error)
	ListMessages(ctx context.Context, sessionId int64, page, size int) ([]*AIMessage, int64, error)
	ListAllMessages(ctx context.Context, sessionId int64) ([]*AIMessage, error)
	FindMessageForUser(ctx context.Context, sessionId, messageId int64, userId string) (*AIMessage, error)
	CreateTurn(ctx context.Context, sessionId int64, userId, content, inputJSON, attachmentsJSON string) (int64, error)
	UpdateMessageContent(ctx context.Context, id int64, content string) error
	CompleteMessage(ctx context.Context, id int64, meta CompletionMeta) error
	FailMessage(ctx context.Context, id int64, message string) error
	PrepareRetry(ctx context.Context, sessionId, messageId int64, userId string) (bool, error)
	RecoverPending(ctx context.Context) error
}
type conversationModel struct{ conn sqlx.SqlConn }

func NewConversationModel(conn sqlx.SqlConn) ConversationModel { return &conversationModel{conn: conn} }

const sessionRows = "id,session_no,user_id,skill_code,selection_mode,skill_version,title,status,create_time,update_time"
const messageRows = "id,session_id,role,content,COALESCE(CAST(input_json AS CHAR),'{}') input_json,COALESCE(CAST(attachments_json AS CHAR),'[]') attachments_json,run_id,tokens,prompt_tokens,completion_tokens,provider,model,cost_micros,finish_reason,status,stage,error_message,retry_count,create_time"
const joinedMessageRows = "m.id,m.session_id,m.role,m.content,COALESCE(CAST(m.input_json AS CHAR),'{}') input_json,COALESCE(CAST(m.attachments_json AS CHAR),'[]') attachments_json,m.run_id,m.tokens,m.prompt_tokens,m.completion_tokens,m.provider,m.model,m.cost_micros,m.finish_reason,m.status,m.stage,m.error_message,m.retry_count,m.create_time"

func (m *conversationModel) CreateSession(ctx context.Context, userId, skillCode, selectionMode, skillVersion, question, inputJSON, attachmentsJSON string) (session *AISession, pendingId int64, err error) {
	err = m.conn.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		title := conversationTitle(question)
		no := fmt.Sprintf("AI%d%06d", time.Now().UnixMilli(), time.Now().Nanosecond()%1000000)
		res, e := tx.ExecCtx(ctx, "INSERT INTO ai_session(session_no,user_id,skill_code,selection_mode,skill_version,title,status) VALUES(?,?,?,?,?,?,?)", no, userId, skillCode, selectionMode, skillVersion, title, SessionStatusActive)
		if e != nil {
			return e
		}
		id, e := res.LastInsertId()
		if e != nil {
			return e
		}
		session = &AISession{Id: id, SessionNo: no, UserId: userId, SkillCode: skillCode, SelectionMode: selectionMode, SkillVersion: skillVersion, Title: title, Status: SessionStatusActive}
		if strings.TrimSpace(question) == "" {
			return nil
		}
		if inputJSON == "" {
			inputJSON = "{}"
		}
		if attachmentsJSON == "" {
			attachmentsJSON = "[]"
		}
		if _, e = tx.ExecCtx(ctx, "INSERT INTO ai_message(session_id,role,content,input_json,attachments_json,status) VALUES(?,?,?,CAST(? AS JSON),CAST(? AS JSON),?)", id, RoleUser, strings.TrimSpace(question), inputJSON, attachmentsJSON, MessageStatusCompleted); e != nil {
			return e
		}
		pending, e := tx.ExecCtx(ctx, "INSERT INTO ai_message(session_id,role,content,status) VALUES(?,?,?,?)", id, RoleAssistant, "", MessageStatusPending)
		if e != nil {
			return e
		}
		pendingId, e = pending.LastInsertId()
		return e
	})
	return
}

func (m *conversationModel) ListSessions(ctx context.Context, userId, status string, page, size int) ([]*AISession, int64, error) {
	where := "user_id=?"
	args := []interface{}{userId}
	if status != "" {
		where += " AND status=?"
		args = append(args, status)
	}
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, "SELECT COUNT(1) FROM ai_session WHERE "+where, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*AISession{}, 0, nil
	}
	args = append(args, (page-1)*size, size)
	var list []*AISession
	err := m.conn.QueryRowsCtx(ctx, &list, "SELECT "+sessionRows+" FROM ai_session WHERE "+where+" ORDER BY update_time DESC,id DESC LIMIT ?,?", args...)
	return list, total, err
}
func (m *conversationModel) FindSession(ctx context.Context, id int64) (*AISession, error) {
	var s AISession
	err := m.conn.QueryRowCtx(ctx, &s, "SELECT "+sessionRows+" FROM ai_session WHERE id=?", id)
	return &s, err
}
func (m *conversationModel) CloseSession(ctx context.Context, id int64, userId string) (bool, error) {
	res, err := m.conn.ExecCtx(ctx, "UPDATE ai_session SET status=? WHERE id=? AND user_id=?", SessionStatusClosed, id, userId)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}
func (m *conversationModel) ListMessages(ctx context.Context, sessionId int64, page, size int) ([]*AIMessage, int64, error) {
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, "SELECT COUNT(1) FROM ai_message WHERE session_id=?", sessionId); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*AIMessage{}, 0, nil
	}
	var list []*AIMessage
	err := m.conn.QueryRowsCtx(ctx, &list, "SELECT "+messageRows+" FROM ai_message WHERE session_id=? ORDER BY id ASC LIMIT ?,?", sessionId, (page-1)*size, size)
	return list, total, err
}
func (m *conversationModel) ListAllMessages(ctx context.Context, sessionId int64) ([]*AIMessage, error) {
	var list []*AIMessage
	err := m.conn.QueryRowsCtx(ctx, &list, "SELECT "+messageRows+" FROM ai_message WHERE session_id=? ORDER BY id ASC", sessionId)
	return list, err
}
func (m *conversationModel) FindMessageForUser(ctx context.Context, sessionId, messageId int64, userId string) (*AIMessage, error) {
	var message AIMessage
	err := m.conn.QueryRowCtx(ctx, &message, "SELECT "+joinedMessageRows+" FROM ai_message m JOIN ai_session s ON s.id=m.session_id WHERE m.id=? AND m.session_id=? AND s.user_id=?", messageId, sessionId, userId)
	return &message, err
}
func (m *conversationModel) CreateTurn(ctx context.Context, sessionId int64, userId, content, inputJSON, attachmentsJSON string) (pendingId int64, err error) {
	err = m.conn.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var s AISession
		if e := tx.QueryRowCtx(ctx, &s, "SELECT "+sessionRows+" FROM ai_session WHERE id=? FOR UPDATE", sessionId); e != nil {
			return e
		}
		if s.UserId != userId {
			return ErrConversationForbidden
		}
		if s.Status != SessionStatusActive {
			return ErrSessionClosed
		}
		if inputJSON == "" {
			inputJSON = "{}"
		}
		if attachmentsJSON == "" {
			attachmentsJSON = "[]"
		}
		if _, e := tx.ExecCtx(ctx, "INSERT INTO ai_message(session_id,role,content,input_json,attachments_json,status) VALUES(?,?,?,CAST(? AS JSON),CAST(? AS JSON),?)", sessionId, RoleUser, strings.TrimSpace(content), inputJSON, attachmentsJSON, MessageStatusCompleted); e != nil {
			return e
		}
		res, e := tx.ExecCtx(ctx, "INSERT INTO ai_message(session_id,role,content,status) VALUES(?,?,?,?)", sessionId, RoleAssistant, "", MessageStatusPending)
		if e != nil {
			return e
		}
		pendingId, e = res.LastInsertId()
		if e != nil {
			return e
		}
		_, e = tx.ExecCtx(ctx, "UPDATE ai_session SET title=IF(title='新对话',?,title),update_time=CURRENT_TIMESTAMP WHERE id=?", conversationTitle(content), sessionId)
		return e
	})
	return
}
func (m *conversationModel) UpdateMessageContent(ctx context.Context, id int64, content string) error {
	_, err := m.conn.ExecCtx(ctx, "UPDATE ai_message SET content=? WHERE id=? AND role=? AND status=?", content, id, RoleAssistant, MessageStatusPending)
	return err
}
func (m *conversationModel) CompleteMessage(ctx context.Context, id int64, meta CompletionMeta) error {
	totalTokens := meta.PromptTokens + meta.CompletionTokens
	_, err := m.conn.ExecCtx(ctx, "UPDATE ai_message SET content=?,tokens=?,prompt_tokens=?,completion_tokens=?,provider=?,model=?,cost_micros=?,finish_reason=?,status=?,stage='completed',error_message='' WHERE id=? AND role=?", meta.Content, totalTokens, meta.PromptTokens, meta.CompletionTokens, meta.Provider, meta.Model, meta.CostMicros, meta.FinishReason, MessageStatusCompleted, id, RoleAssistant)
	return err
}
func (m *conversationModel) FailMessage(ctx context.Context, id int64, message string) error {
	r := []rune(message)
	if len(r) > 255 {
		message = string(r[:255])
	}
	_, err := m.conn.ExecCtx(ctx, "UPDATE ai_message SET status=?,stage='failed',error_message=? WHERE id=? AND role=?", MessageStatusFailed, message, id, RoleAssistant)
	return err
}
func (m *conversationModel) PrepareRetry(ctx context.Context, sessionId, messageId int64, userId string) (bool, error) {
	res, err := m.conn.ExecCtx(ctx, "UPDATE ai_message m JOIN ai_session s ON s.id=m.session_id SET m.status=?,m.stage='',m.run_id=0,m.error_message='',m.retry_count=m.retry_count+1 WHERE m.id=? AND m.session_id=? AND m.role=? AND m.status=? AND s.user_id=?", MessageStatusPending, messageId, sessionId, RoleAssistant, MessageStatusFailed, userId)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	return n > 0, err
}
func (m *conversationModel) RecoverPending(ctx context.Context) error {
	_, err := m.conn.ExecCtx(ctx, "UPDATE ai_message SET status=?,error_message='服务重启，点击重试' WHERE role=? AND status=?", MessageStatusFailed, RoleAssistant, MessageStatusPending)
	return err
}

func conversationTitle(question string) string {
	q := strings.TrimSpace(question)
	if q == "" {
		return "新对话"
	}
	r := []rune(q)
	if len(r) > 30 {
		r = r[:30]
	}
	return string(r)
}

package model

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	RunStatusRunning   = "running"
	RunStatusCompleted = "completed"
	RunStatusFailed    = "failed"
)

type AIRun struct {
	Id              int64  `db:"id"`
	RunNo           string `db:"run_no"`
	SessionId       int64  `db:"session_id"`
	MessageId       int64  `db:"message_id"`
	UserId          string `db:"user_id"`
	SkillCode       string `db:"skill_code"`
	SkillVersion    string `db:"skill_version"`
	SelectionMode   string `db:"selection_mode"`
	Provider        string `db:"provider"`
	Model           string `db:"model"`
	Status          string `db:"status"`
	Stage           string `db:"stage"`
	ReasoningTokens int    `db:"reasoning_tokens"`
	ErrorMessage    string `db:"error_message"`
	StartedAt       string `db:"started_at"`
	CompletedAt     string `db:"completed_at"`
}

type AIToolCall struct {
	Id               int64  `db:"id"`
	RunId            int64  `db:"run_id"`
	ServerCode       string `db:"server_code"`
	ToolName         string `db:"tool_name"`
	ArgumentsSummary string `db:"arguments_summary"`
	ResultSummary    string `db:"result_summary"`
	Status           string `db:"status"`
	LatencyMs        int    `db:"latency_ms"`
	ErrorMessage     string `db:"error_message"`
	CreatedAt        string `db:"create_time"`
	CompletedAt      string `db:"complete_time"`
}

type RunModel interface {
	Start(ctx context.Context, session AISession, messageId int64, provider, providerModel string) (*AIRun, error)
	UpdateStage(ctx context.Context, runId, messageId int64, stage string) error
	Complete(ctx context.Context, runId int64, reasoningTokens int, providerModel string) error
	Fail(ctx context.Context, runId int64, message string) error
	StartTool(ctx context.Context, runId int64, server, tool, argumentsSummary string) (int64, error)
	CompleteTool(ctx context.Context, id int64, resultSummary string, latencyMs int) error
	FailTool(ctx context.Context, id int64, message string, latencyMs int) error
	TraceForUser(ctx context.Context, messageId int64, userId string) (*AIRun, []*AIToolCall, error)
}

type runModel struct{ conn sqlx.SqlConn }

func NewRunModel(conn sqlx.SqlConn) RunModel { return &runModel{conn: conn} }

const runRows = "r.id,r.run_no,r.session_id,r.message_id,r.user_id,r.skill_code,r.skill_version,r.selection_mode,r.provider,r.model,r.status,r.stage,r.reasoning_tokens,r.error_message,r.started_at,COALESCE(CAST(r.completed_at AS CHAR),'') completed_at"

func (m *runModel) Start(ctx context.Context, session AISession, messageId int64, providerName, providerModel string) (*AIRun, error) {
	var run AIRun
	err := m.conn.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		runNo := fmt.Sprintf("AIR%d%06d", time.Now().UnixMilli(), time.Now().Nanosecond()%1000000)
		res, err := tx.ExecCtx(ctx, `INSERT INTO ai_run(run_no,session_id,message_id,user_id,skill_code,skill_version,selection_mode,provider,model,status,stage)
			VALUES(?,?,?,?,?,?,?,?,?,?,?)`, runNo, session.Id, messageId, session.UserId, session.SkillCode, session.SkillVersion, session.SelectionMode, providerName, providerModel, RunStatusRunning, "accepted")
		if err != nil {
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		run = AIRun{Id: id, RunNo: runNo, SessionId: session.Id, MessageId: messageId, UserId: session.UserId, SkillCode: session.SkillCode, SkillVersion: session.SkillVersion, SelectionMode: session.SelectionMode, Provider: providerName, Model: providerModel, Status: RunStatusRunning, Stage: "accepted"}
		_, err = tx.ExecCtx(ctx, "UPDATE ai_message SET run_id=?,stage=? WHERE id=?", id, "accepted", messageId)
		return err
	})
	return &run, err
}

func (m *runModel) UpdateStage(ctx context.Context, runId, messageId int64, stage string) error {
	_, err := m.conn.ExecCtx(ctx, "UPDATE ai_run r JOIN ai_message m ON m.id=r.message_id SET r.stage=?,m.stage=? WHERE r.id=? AND m.id=?", stage, stage, runId, messageId)
	return err
}

func (m *runModel) Complete(ctx context.Context, runId int64, reasoningTokens int, providerModel string) error {
	_, err := m.conn.ExecCtx(ctx, "UPDATE ai_run SET status=?,stage='completed',reasoning_tokens=?,model=?,completed_at=CURRENT_TIMESTAMP(3) WHERE id=?", RunStatusCompleted, reasoningTokens, providerModel, runId)
	return err
}

func (m *runModel) Fail(ctx context.Context, runId int64, message string) error {
	message = shortError(message)
	_, err := m.conn.ExecCtx(ctx, "UPDATE ai_run SET status=?,stage='failed',error_message=?,completed_at=CURRENT_TIMESTAMP(3) WHERE id=?", RunStatusFailed, message, runId)
	return err
}

func (m *runModel) StartTool(ctx context.Context, runId int64, server, tool, argumentsSummary string) (int64, error) {
	res, err := m.conn.ExecCtx(ctx, "INSERT INTO ai_tool_call(run_id,server_code,tool_name,arguments_summary,status) VALUES(?,?,?,CAST(? AS JSON),'running')", runId, server, tool, argumentsSummary)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (m *runModel) CompleteTool(ctx context.Context, id int64, resultSummary string, latencyMs int) error {
	_, err := m.conn.ExecCtx(ctx, "UPDATE ai_tool_call SET result_summary=?,status='completed',latency_ms=?,complete_time=CURRENT_TIMESTAMP(3) WHERE id=?", truncate(resultSummary, 4000), latencyMs, id)
	return err
}

func (m *runModel) FailTool(ctx context.Context, id int64, message string, latencyMs int) error {
	_, err := m.conn.ExecCtx(ctx, "UPDATE ai_tool_call SET status='failed',latency_ms=?,error_message=?,complete_time=CURRENT_TIMESTAMP(3) WHERE id=?", latencyMs, shortError(message), id)
	return err
}

func (m *runModel) TraceForUser(ctx context.Context, messageId int64, userId string) (*AIRun, []*AIToolCall, error) {
	var run AIRun
	if err := m.conn.QueryRowCtx(ctx, &run, "SELECT "+runRows+" FROM ai_run r JOIN ai_session s ON s.id=r.session_id WHERE r.message_id=? AND s.user_id=? ORDER BY r.id DESC LIMIT 1", messageId, userId); err != nil {
		return nil, nil, err
	}
	var tools []*AIToolCall
	err := m.conn.QueryRowsCtx(ctx, &tools, `SELECT id,run_id,server_code,tool_name,COALESCE(CAST(arguments_summary AS CHAR),'{}') arguments_summary,
		COALESCE(result_summary,'') result_summary,status,latency_ms,error_message,create_time,COALESCE(CAST(complete_time AS CHAR),'') complete_time
		FROM ai_tool_call WHERE run_id=? ORDER BY id`, run.Id)
	return &run, tools, err
}

func shortError(value string) string { return truncate(strings.TrimSpace(value), 255) }

func truncate(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return string(runes[:max])
}

package model

import "errors"

const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
)
const (
	MessageStatusPending   = "pending"
	MessageStatusCompleted = "completed"
	MessageStatusFailed    = "failed"
)

var (
	ErrConversationForbidden = errors.New("conversation forbidden")
	ErrSessionClosed         = errors.New("session closed")
)

type AIMessage struct {
	Id               int64  `db:"id"`
	SessionId        int64  `db:"session_id"`
	Role             string `db:"role"`
	Content          string `db:"content"`
	InputJSON        string `db:"input_json"`
	Tokens           int    `db:"tokens"`
	PromptTokens     int    `db:"prompt_tokens"`
	CompletionTokens int    `db:"completion_tokens"`
	Provider         string `db:"provider"`
	Model            string `db:"model"`
	CostMicros       int64  `db:"cost_micros"`
	FinishReason     string `db:"finish_reason"`
	Status           string `db:"status"`
	ErrorMessage     string `db:"error_message"`
	RetryCount       int    `db:"retry_count"`
	CreatedAt        string `db:"create_time"`
}

type CompletionMeta struct {
	Content          string
	PromptTokens     int
	CompletionTokens int
	Provider         string
	Model            string
	CostMicros       int64
	FinishReason     string
}

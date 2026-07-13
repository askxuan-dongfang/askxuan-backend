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
	Id           int64  `db:"id"`
	SessionId    int64  `db:"session_id"`
	Role         string `db:"role"`
	Content      string `db:"content"`
	Tokens       int    `db:"tokens"`
	Status       string `db:"status"`
	ErrorMessage string `db:"error_message"`
	RetryCount   int    `db:"retry_count"`
	CreatedAt    string `db:"create_time"`
}

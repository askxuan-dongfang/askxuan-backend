package types

import "encoding/json"

// AISkill AI 问事技能
type AISkill struct {
	Id           int64           `json:"id"`
	Code         string          `json:"code"`
	Category     string          `json:"category"`
	Name         string          `json:"name"`
	Version      string          `json:"version"`
	Description  string          `json:"description"`
	Icon         string          `json:"icon"`
	SourceType   string          `json:"sourceType"`
	SourceRef    string          `json:"sourceRef"`
	InputSchema  json.RawMessage `json:"inputSchema"`
	Capabilities json.RawMessage `json:"capabilities"`
	RiskLevel    string          `json:"riskLevel"`
	SortOrder    int             `json:"sortOrder"`
	Status       string          `json:"status"`
	CreatedAt    string          `json:"createdAt"`
}

// SkillListReq 技能列表请求
type SkillListReq struct {
	Status string `form:"status,optional"`
}

// SkillListResp 技能列表响应
type SkillListResp struct {
	List []AISkill `json:"list"`
}

// IdResp 通用 ID 响应
type IdResp struct {
	Id int64 `json:"id"`
}

// AISession 对话会话
type AISession struct {
	Id        int64  `json:"id"`
	SessionNo string `json:"sessionNo"`
	UserId    string `json:"userId"`
	SkillCode string `json:"skillCode"`
	Title     string `json:"title"`
	Status    string `json:"status"` // active/closed
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

// SessionCreateReq 创建会话请求
type SessionCreateReq struct {
	UserId    string                 `json:"userId,optional"`
	SkillCode string                 `json:"skillCode,optional"`
	Question  string                 `json:"question,optional"`
	Inputs    map[string]interface{} `json:"inputs,optional"`
}

// SessionCreateResp 创建会话响应
type SessionCreateResp struct {
	Id        int64  `json:"id"`
	SessionNo string `json:"sessionNo"`
	SkillCode string `json:"skillCode"`
	Status    string `json:"status"`
	MessageId int64  `json:"messageId"`
}

// SessionListReq 会话列表请求
type SessionListReq struct {
	UserId string `form:"userId,optional"`
	Status string `form:"status,optional"`
	Page   int    `form:"page,default=1"`
	Size   int    `form:"size,default=20"`
}

// SessionListResp 会话列表响应
type SessionListResp struct {
	Total int64       `json:"total"`
	List  []AISession `json:"list"`
	Page  int         `json:"page"`
	Size  int         `json:"size"`
}

// SessionDetailReq 会话详情请求
type SessionDetailReq struct {
	Id     int64  `path:"id"`
	UserId string `form:"userId,optional"`
}

// AIMessage 对话消息
type AIMessage struct {
	Id               int64                  `json:"id"`
	SessionId        int64                  `json:"sessionId"`
	Role             string                 `json:"role"` // user/assistant
	Content          string                 `json:"content"`
	Inputs           map[string]interface{} `json:"inputs,omitempty"`
	Tokens           int                    `json:"tokens"`
	PromptTokens     int                    `json:"promptTokens"`
	CompletionTokens int                    `json:"completionTokens"`
	Provider         string                 `json:"provider"`
	Model            string                 `json:"model"`
	CostMicros       int64                  `json:"costMicros"`
	FinishReason     string                 `json:"finishReason"`
	Status           string                 `json:"status"`
	ErrorMessage     string                 `json:"errorMessage"`
	Retryable        bool                   `json:"retryable"`
	CreatedAt        string                 `json:"createdAt"`
}

// SessionDetailResp 会话详情响应（含消息列表）
type SessionDetailResp struct {
	Session  AISession   `json:"session"`
	Messages []AIMessage `json:"messages"`
}

// MessageListReq 会话消息列表请求
type MessageListReq struct {
	Id     int64  `path:"id"`
	UserId string `form:"userId,optional"`
	Page   int    `form:"page,default=1"`
	Size   int    `form:"size,default=20"`
}

// MessageListResp 会话消息列表响应
type MessageListResp struct {
	Total int64       `json:"total"`
	List  []AIMessage `json:"list"`
	Page  int         `json:"page"`
	Size  int         `json:"size"`
}

// SessionDeleteReq 关闭会话请求
type SessionDeleteReq struct {
	Id     int64  `path:"id"`
	UserId string `form:"userId,optional"`
}

// MessageSendReq 发送问事消息请求
type MessageSendReq struct {
	Id      int64                  `path:"id"` // 会话 ID
	UserId  string                 `json:"userId,optional"`
	Content string                 `json:"content"`
	Inputs  map[string]interface{} `json:"inputs,optional"`
}

// MessageSendResp 发送消息响应（异步处理，返回受理状态）
type MessageSendResp struct {
	SessionId int64  `json:"sessionId"`
	MessageId int64  `json:"messageId"`
	Status    string `json:"status"` // pending/completed
}

type MessageRetryReq struct {
	Id        int64  `path:"id"`
	MessageId int64  `path:"messageId"`
	UserId    string `json:"userId,optional"`
}

type MessageStreamReq struct {
	Id        int64  `path:"id"`
	MessageId int64  `path:"messageId"`
	UserId    string `form:"userId,optional"`
}

type UsageSummaryReq struct {
	UserId string `form:"userId,optional"`
}

type UsageSummaryResp struct {
	MinuteRequests  int   `json:"minuteRequests"`
	MinuteLimit     int   `json:"minuteLimit"`
	DailyRequests   int   `json:"dailyRequests"`
	DailyLimit      int   `json:"dailyLimit"`
	DailyTokens     int64 `json:"dailyTokens"`
	DailyCostMicros int64 `json:"dailyCostMicros"`
}

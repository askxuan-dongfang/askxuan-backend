package im

// OpenIM API 请求/响应结构体

// GetAdminTokenReq 获取管理员 token 请求
type GetAdminTokenReq struct {
	Secret string `json:"secret"`
	UserID string `json:"userID"`
}

// GetAdminTokenResp 获取管理员 token 响应
type GetAdminTokenResp struct {
	Token      string `json:"token"`
	ExpireTime int64  `json:"expireTimeSeconds"`
}

// UserRegisterReq 用户注册请求
type UserRegisterReq struct {
	Users []OpenIMUser `json:"users"`
}

type OpenIMUser struct {
	UserID   string `json:"userID"`
	Nickname string `json:"nickname"`
	FaceURL  string `json:"faceURL"`
}

// UserTokenReq 获取用户 token 请求
type UserTokenReq struct {
	UserID     string `json:"userID"`
	PlatformID int    `json:"platformID"`
}

// UserTokenResp 获取用户 token 响应
type UserTokenResp struct {
	Token      string `json:"token"`
	ExpireTime int64  `json:"expireTimeSeconds"`
}

// SendMsgReq 发送消息请求
type SendMsgReq struct {
	SendID           string `json:"sendID"`
	RecvID           string `json:"recvID"`
	GroupID          string `json:"groupID,omitempty"`
	SenderName       string `json:"senderNickname"`
	SenderPlatformID int    `json:"senderPlatformID"`
	SessionType      int    `json:"sessionType"` // 1=单聊
	ContentType      int    `json:"contentType"` // 101=文本
	Content          any    `json:"content"`
	Ex               string `json:"ex,omitempty"`
}

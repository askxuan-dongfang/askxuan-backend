package handler

import (
	"encoding/json"
	"net/http"

	"github.com/askxuan/message-service/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// OpenIMWebhookReq OpenIM webhook 事件请求体
type OpenIMWebhookReq struct {
	SendID      string `json:"sendID"`
	RecvID      string `json:"recvID"`
	Content     string `json:"content"`
	SessionType int    `json:"sessionType"`
	ContentType int    `json:"contentType"`
	SenderName  string `json:"senderName"`
	SenderNick  string `json:"senderNickname"`
}

// OpenIMWebhookHandler is retained for old OpenIM configurations only.
// Paid booking chat is handled by booking-service; this public compatibility
// endpoint deliberately performs no writes so it cannot forge consult notices.
func OpenIMWebhookHandler(_ *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req OpenIMWebhookReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		httpx.OkJsonCtx(r.Context(), w, map[string]any{"code": 0})
	}
}

package handler

import (
	"encoding/json"
	"net/http"

	"github.com/askxuan/message-service/internal/model"
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
}

// OpenIMWebhookHandler 接收 OpenIM afterSendSingleMsg 事件回调
// 落库到 message 表（biz_type="consult"），供未集成 SDK 的端（mobile-customer）轮询兜底
func OpenIMWebhookHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req OpenIMWebhookReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		// 仅处理单聊文本消息（sessionType=1, contentType=101）
		if req.SessionType == 1 && req.ContentType == 101 && req.RecvID != "" {
			title := "新的咨询消息"
			if req.SenderName != "" {
				title = "来自 " + req.SenderName + " 的消息"
			}
			_, _ = svcCtx.MessageModel.Insert(r.Context(), &model.Message{
				UserId:  req.RecvID,
				Title:   title,
				Content: req.Content,
				BizType: "consult",
				BizId:   req.SendID,
				IsRead:  0,
			})
		}

		httpx.OkJsonCtx(r.Context(), w, map[string]any{"code": 0})
	}
}

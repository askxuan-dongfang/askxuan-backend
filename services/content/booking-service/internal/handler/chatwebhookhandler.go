package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/askxuan/booking-service/internal/logic"
	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/core/logx"
)

type openIMBookingChatWebhookReq struct {
	CallbackCommand string `json:"callbackCommand"`
	SendID          string `json:"sendID"`
	RecvID          string `json:"recvID"`
	ClientMsgID     string `json:"clientMsgID"`
	ServerMsgID     string `json:"serverMsgID"`
	Content         string `json:"content"`
	Ex              string `json:"ex"`
}

type openIMBookingChatWebhookResp struct {
	ActionCode int32  `json:"actionCode"`
	ErrCode    int32  `json:"errCode,omitempty"`
	ErrMsg     string `json:"errMsg,omitempty"`
	NextCode   int32  `json:"nextCode,omitempty"`
}

func bookingChatWebhookHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req openIMBookingChatWebhookReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeOpenIMWebhookResp(w, openIMBookingChatWebhookResp{ActionCode: 0, ErrCode: int32(common.ErrParam.Code), ErrMsg: common.ErrParam.Msg, NextCode: 1})
			return
		}
		command := req.CallbackCommand
		if command == "" {
			command = strings.TrimPrefix(r.URL.Path, "/openim/booking-chat-webhook/")
		}
		if strings.Contains(strings.ToLower(command), "beforesendsinglemsg") {
			if _, err := logic.AuthorizeOpenIMMessage(r.Context(), svcCtx, req.SendID, req.RecvID, req.Ex); err != nil {
				code, message := int32(common.ErrBookingChatUnavailable.Code), common.ErrBookingChatUnavailable.Msg
				if bizErr, ok := err.(*common.BizError); ok {
					code, message = int32(bizErr.Code), bizErr.Msg
				}
				writeOpenIMWebhookResp(w, openIMBookingChatWebhookResp{ActionCode: 0, ErrCode: code, ErrMsg: message, NextCode: 1})
				return
			}
			writeOpenIMWebhookResp(w, openIMBookingChatWebhookResp{ActionCode: 0})
			return
		}
		if strings.Contains(strings.ToLower(command), "aftersendsinglemsg") {
			if err := logic.RecordOpenIMMessage(r.Context(), svcCtx, logic.OpenIMCallbackMessage{
				SendID: req.SendID, RecvID: req.RecvID, ClientMsgID: req.ClientMsgID,
				ServerMsgID: req.ServerMsgID, Content: req.Content, Ex: req.Ex,
			}); err != nil {
				logx.WithContext(r.Context()).Errorf("记录 OpenIM 预约聊天回调失败: %v", err)
			}
		}
		writeOpenIMWebhookResp(w, openIMBookingChatWebhookResp{ActionCode: 0})
	}
}

func writeOpenIMWebhookResp(w http.ResponseWriter, resp openIMBookingChatWebhookResp) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(resp)
}

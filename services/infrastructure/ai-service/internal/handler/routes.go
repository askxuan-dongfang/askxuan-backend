package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/askxuan/ai-service/internal/logic"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/askxuan/ai-service/internal/types"
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"

	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// RegisterHandlers 注册 ai 服务路由
func RegisterHandlers(server *rest.Server, svcCtx *svc.ServiceContext) {
	server.Use(middleware.CorsFunc)

	server.AddRoutes([]rest.Route{
		{Method: http.MethodGet, Path: "/api/v1/ai/skills", Handler: skillListHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/ai/sessions", Handler: sessionCreateHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/ai/sessions", Handler: sessionListHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/ai/sessions/:id", Handler: sessionDetailHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/ai/sessions/:id/messages", Handler: messageListHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/ai/sessions/:id/messages", Handler: messageSendHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/ai/sessions/:id/messages/:messageId/retry", Handler: messageRetryHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/ai/sessions/:id/messages/:messageId/stream", Handler: messageStreamHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/ai/sessions/:id/messages/:messageId/trace", Handler: messageTraceHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/ai/usage", Handler: usageSummaryHandler(svcCtx)},
		{Method: http.MethodDelete, Path: "/api/v1/ai/sessions/:id", Handler: sessionDeleteHandler(svcCtx)},
	})
}

func skillListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SkillListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.NewSkillListLogic(r.Context(), svcCtx).SkillList(&req)
		respond(w, resp, err)
	}
}

func sessionCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SessionCreateReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		if userId, err := resolveUserID(r, req.UserId); err != nil {
			common.JsonError(w, err)
			return
		} else {
			req.UserId = userId
		}
		resp, err := logic.NewSessionCreateLogic(r.Context(), svcCtx).Create(&req)
		respond(w, resp, err)
	}
}

func sessionListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SessionListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		if userId, err := resolveUserID(r, req.UserId); err != nil {
			common.JsonError(w, err)
			return
		} else {
			req.UserId = userId
		}
		resp, err := logic.NewSessionListLogic(r.Context(), svcCtx).List(&req)
		respond(w, resp, err)
	}
}

func sessionDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SessionDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		if userId, err := resolveUserID(r, req.UserId); err != nil {
			common.JsonError(w, err)
			return
		} else {
			req.UserId = userId
		}
		resp, err := logic.NewSessionDetailLogic(r.Context(), svcCtx).Detail(&req)
		respond(w, resp, err)
	}
}

func messageListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.MessageListReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		if userId, err := resolveUserID(r, req.UserId); err != nil {
			common.JsonError(w, err)
			return
		} else {
			req.UserId = userId
		}
		resp, err := logic.NewMessageListLogic(r.Context(), svcCtx).List(&req)
		respond(w, resp, err)
	}
}

func messageSendHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.MessageSendReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		if userId, err := resolveUserID(r, req.UserId); err != nil {
			common.JsonError(w, err)
			return
		} else {
			req.UserId = userId
		}
		resp, err := logic.NewMessageSendLogic(r.Context(), svcCtx).Send(&req)
		respond(w, resp, err)
	}
}

func sessionDeleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SessionDeleteReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		if userId, err := resolveUserID(r, req.UserId); err != nil {
			common.JsonError(w, err)
			return
		} else {
			req.UserId = userId
		}
		resp, err := logic.NewSessionDeleteLogic(r.Context(), svcCtx).Delete(&req)
		respond(w, resp, err)
	}
}

func messageRetryHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.MessageRetryReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		if userId, err := resolveUserID(r, req.UserId); err != nil {
			common.JsonError(w, err)
			return
		} else {
			req.UserId = userId
		}
		resp, err := logic.NewMessageRetryLogic(r.Context(), svcCtx).Retry(&req)
		respond(w, resp, err)
	}
}

func usageSummaryHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.UsageSummaryReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		userID, err := resolveUserID(r, req.UserId)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		req.UserId = userID
		resp, err := logic.NewUsageSummaryLogic(r.Context(), svcCtx).Summary(&req)
		respond(w, resp, err)
	}
}

func messageTraceHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.MessageTraceReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		userID, err := resolveUserID(r, req.UserId)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		req.UserId = userID
		resp, err := logic.NewMessageTraceLogic(r.Context(), svcCtx).Trace(&req)
		respond(w, resp, err)
	}
}

func messageStreamHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.MessageStreamReq
		if err := httpx.Parse(r, &req); err != nil || req.Id == 0 || req.MessageId == 0 {
			common.JsonError(w, common.ErrParam)
			return
		}
		userID, err := resolveUserID(r, req.UserId)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			common.JsonError(w, common.ErrSystem)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")

		lastContent, lastStage := "", ""
		ticker := time.NewTicker(180 * time.Millisecond)
		defer ticker.Stop()
		timeout := time.NewTimer(70 * time.Second)
		defer timeout.Stop()
		for {
			message, err := svcCtx.ConversationModel.FindMessageForUser(r.Context(), req.Id, req.MessageId, userID)
			if err != nil {
				writeSSE(w, flusher, "error", map[string]interface{}{"message": "消息不存在或无权访问"})
				return
			}
			if message.Content != lastContent {
				delta := message.Content
				if len(message.Content) >= len(lastContent) && message.Content[:len(lastContent)] == lastContent {
					delta = message.Content[len(lastContent):]
				}
				writeSSE(w, flusher, "delta", map[string]interface{}{"messageId": message.Id, "content": delta, "snapshot": message.Content})
				lastContent = message.Content
			}
			if message.Stage != "" && message.Stage != lastStage {
				writeSSE(w, flusher, "stage", map[string]interface{}{"messageId": message.Id, "runId": message.RunId, "stage": message.Stage})
				lastStage = message.Stage
			}
			switch message.Status {
			case "completed":
				writeSSE(w, flusher, "done", map[string]interface{}{
					"messageId": message.Id, "content": message.Content, "status": message.Status,
					"promptTokens": message.PromptTokens, "completionTokens": message.CompletionTokens,
					"provider": message.Provider, "model": message.Model, "costMicros": message.CostMicros, "runId": message.RunId, "stage": message.Stage,
				})
				return
			case "failed":
				writeSSE(w, flusher, "error", map[string]interface{}{"messageId": message.Id, "message": message.ErrorMessage, "retryable": true})
				return
			}
			select {
			case <-r.Context().Done():
				return
			case <-timeout.C:
				writeSSE(w, flusher, "timeout", map[string]interface{}{"messageId": message.Id, "status": message.Status})
				return
			case <-ticker.C:
			}
		}
	}
}

func writeSSE(w http.ResponseWriter, flusher http.Flusher, event string, payload interface{}) {
	data, _ := json.Marshal(payload)
	_, _ = fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
	flusher.Flush()
}

func resolveUserID(r *http.Request, requested string) (string, error) {
	trusted := r.Header.Get("X-User-Id")
	if trusted == "" {
		return "", common.ErrForbidden
	}
	if requested != "" && requested != trusted {
		return "", common.ErrForbidden
	}
	return trusted, nil
}

func respond(w http.ResponseWriter, resp interface{}, err error) {
	if err != nil {
		common.JsonError(w, err)
	} else {
		common.Ok(w, resp)
	}
}

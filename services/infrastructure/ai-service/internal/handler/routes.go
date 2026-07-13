package handler

import (
	"net/http"

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

func resolveUserID(r *http.Request, requested string) (string, error) {
	trusted := r.Header.Get("X-User-Id")
	if trusted != "" {
		if requested != "" && requested != trusted {
			return "", common.ErrForbidden
		}
		return trusted, nil
	}
	if requested == "" {
		return "", common.ErrForbidden
	}
	return requested, nil
}

func respond(w http.ResponseWriter, resp interface{}, err error) {
	if err != nil {
		common.JsonError(w, err)
	} else {
		common.Ok(w, resp)
	}
}

package handler

import (
	"crypto/subtle"
	"net/http"

	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/media-service/internal/logic"
	"github.com/askxuan/media-service/internal/svc"
	"github.com/askxuan/media-service/internal/types"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func RegisterHandlers(server *rest.Server, svcCtx *svc.ServiceContext) {
	server.Use(middleware.CorsFunc)
	server.AddRoutes([]rest.Route{
		{Method: http.MethodPost, Path: "/api/v1/media/uploads/credentials", Handler: uploadCredentialHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/media/:id/complete", Handler: uploadCompleteHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/media/:id", Handler: mediaDetailHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/media/callback/transcode", Handler: transcodeCallbackHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/media/callback/audit", Handler: auditCallbackHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/live/capabilities", Handler: liveCapabilitiesHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/live/rooms", Handler: liveRoomCreateHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/live/rooms", Handler: liveRoomListHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/live/rooms/:id", Handler: liveRoomDetailHandler(svcCtx)},
		{Method: http.MethodPut, Path: "/api/v1/live/rooms/:id/openim", Handler: liveBindOpenIMHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/live/rooms/:id/start", Handler: liveStartHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/live/rooms/:id/close", Handler: liveCloseHandler(svcCtx)},
	})
}

func uploadCredentialHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.UploadCredentialReq
		if !parse(w, r, &req) {
			return
		}
		owner, err := resolveOwner(r, req.UserId)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		req.UserId = owner
		resp, err := logic.CreateUploadCredential(r.Context(), svcCtx, &req)
		respond(w, resp, err)
	}
}

func uploadCompleteHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.UploadCompleteReq
		if !parse(w, r, &req) {
			return
		}
		owner, err := resolveOwner(r, req.UserId)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		req.UserId = owner
		resp, err := logic.CompleteUpload(r.Context(), svcCtx, &req)
		respond(w, resp, err)
	}
}

func mediaDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.MediaDetailReq
		if !parse(w, r, &req) {
			return
		}
		resp, err := logic.GetMedia(r.Context(), svcCtx, req.Id, r.Header.Get("X-User-Id"))
		respond(w, resp, err)
	}
}

func transcodeCallbackHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !validCallbackToken(r, svcCtx.CallbackToken) {
			common.JsonError(w, common.ErrForbidden)
			return
		}
		var req types.MediaCallbackReq
		if !parse(w, r, &req) {
			return
		}
		resp, err := logic.TranscodeCallback(r.Context(), svcCtx, &req)
		respond(w, resp, err)
	}
}

func auditCallbackHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !validCallbackToken(r, svcCtx.CallbackToken) {
			common.JsonError(w, common.ErrForbidden)
			return
		}
		var req types.AuditCallbackReq
		if !parse(w, r, &req) {
			return
		}
		resp, err := logic.AuditCallback(r.Context(), svcCtx, &req)
		respond(w, resp, err)
	}
}

func liveCapabilitiesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) { common.Ok(w, logic.LiveCapabilities(svcCtx)) }
}

func liveRoomCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.LiveRoomCreateReq
		if !parse(w, r, &req) {
			return
		}
		owner, master, err := resolveMaster(r, req.OwnerId, req.MasterId)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		req.OwnerId, req.MasterId = owner, master
		resp, err := logic.CreateLiveRoom(r.Context(), svcCtx, &req)
		respond(w, resp, err)
	}
}

func liveRoomListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.LiveRoomListReq
		if !parse(w, r, &req) {
			return
		}
		resp, err := logic.ListLiveRooms(r.Context(), svcCtx, &req)
		respond(w, resp, err)
	}
}

func liveRoomDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.LiveRoomDetailReq
		if !parse(w, r, &req) {
			return
		}
		resp, err := logic.GetLiveRoom(r.Context(), svcCtx, req.Id, r.Header.Get("X-User-Id"))
		respond(w, resp, err)
	}
}

func liveBindOpenIMHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.LiveBindOpenIMReq
		if !parse(w, r, &req) {
			return
		}
		owner, err := resolveOwner(r, req.OwnerId)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		req.OwnerId = owner
		resp, err := logic.BindLiveOpenIM(r.Context(), svcCtx, &req)
		respond(w, resp, err)
	}
}

func liveStartHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return liveActionHandler(svcCtx, true)
}
func liveCloseHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return liveActionHandler(svcCtx, false)
}

func liveActionHandler(svcCtx *svc.ServiceContext, start bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.LiveRoomActionReq
		if !parse(w, r, &req) {
			return
		}
		owner, err := resolveOwner(r, req.OwnerId)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		req.OwnerId = owner
		if start {
			resp, err := logic.StartLiveRoom(r.Context(), svcCtx, &req)
			respond(w, resp, err)
			return
		}
		resp, err := logic.CloseLiveRoom(r.Context(), svcCtx, &req)
		respond(w, resp, err)
	}
}

func resolveOwner(r *http.Request, requested string) (string, error) {
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

func resolveMaster(r *http.Request, requestedOwner, requestedMaster string) (string, string, error) {
	owner, err := resolveOwner(r, requestedOwner)
	if err != nil {
		return "", "", err
	}
	trustedMaster := r.Header.Get("X-Master-Id")
	if r.Header.Get("X-User-Id") != "" {
		if trustedMaster == "" || (requestedMaster != "" && requestedMaster != trustedMaster) {
			return "", "", common.ErrRoleForbidden
		}
		return owner, trustedMaster, nil
	}
	if requestedMaster == "" {
		return "", "", common.ErrRoleForbidden
	}
	return owner, requestedMaster, nil
}

func validCallbackToken(r *http.Request, expected string) bool {
	actual := r.Header.Get("X-Media-Callback-Token")
	return expected != "" && len(actual) == len(expected) && subtle.ConstantTimeCompare([]byte(actual), []byte(expected)) == 1
}

func parse(w http.ResponseWriter, r *http.Request, req interface{}) bool {
	if err := httpx.Parse(r, req); err != nil {
		common.JsonError(w, common.ErrParam)
		return false
	}
	return true
}

func respond(w http.ResponseWriter, resp interface{}, err error) {
	if err != nil {
		common.JsonError(w, err)
		return
	}
	common.Ok(w, resp)
}

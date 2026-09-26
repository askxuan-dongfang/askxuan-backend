package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/file-service/internal/logic"
	"github.com/askxuan/file-service/internal/svc"
	"github.com/askxuan/file-service/internal/types"

	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// RegisterHandlers 注册 file 服务路由
func RegisterHandlers(server *rest.Server, svcCtx *svc.ServiceContext) {
	server.Use(middleware.CorsFunc)

	server.AddRoutes([]rest.Route{
		{
			Method:  http.MethodPost,
			Path:    "/api/v1/files/upload",
			Handler: uploadHandler(svcCtx),
		},
		{Method: http.MethodGet, Path: "/api/v1/admin/files/backups", Handler: backupListHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/admin/files/backups", Handler: backupCreateHandler(svcCtx)},
		{Method: http.MethodGet, Path: "/api/v1/admin/files/backups/:filename/download", Handler: backupDownloadHandler(svcCtx)},
		{Method: http.MethodPost, Path: "/api/v1/admin/files/backups/:filename/restore", Handler: backupRestoreHandler(svcCtx)},
	})
}

func requirePlatformSuper(w http.ResponseWriter, r *http.Request, secret string) bool {
	claims := accessClaims(r, secret)
	if claims != nil && claims.UserType == "admin" && claims.HasRole("platform_super") {
		return true
	}
	common.JsonError(w, common.ErrRoleForbidden)
	return false
}

func accessClaims(r *http.Request, secret string) *common.CustomClaims {
	parts := strings.Fields(r.Header.Get("Authorization"))
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		claims, err := common.ParseToken(secret, parts[1])
		if err == nil && claims.Type == "access" && claims.UserId > 0 {
			return claims
		}
	}
	return nil
}

func backupListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requirePlatformSuper(w, r, svcCtx.Config.AuthSecret) {
			return
		}
		resp, err := logic.ListBackups(r.Context(), svcCtx)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, resp)
	}
}

func backupCreateHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requirePlatformSuper(w, r, svcCtx.Config.AuthSecret) {
			return
		}
		resp, err := logic.CreateBackup(r.Context(), svcCtx)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, resp)
	}
}

func backupDownloadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requirePlatformSuper(w, r, svcCtx.Config.AuthSecret) {
			return
		}
		var path struct {
			Filename string `path:"filename"`
		}
		if err := httpx.ParsePath(r, &path); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.BackupDownload(r.Context(), svcCtx, path.Filename)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, resp)
	}
}

func backupRestoreHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !requirePlatformSuper(w, r, svcCtx.Config.AuthSecret) {
			return
		}
		var path struct {
			Filename string `path:"filename"`
		}
		if err := httpx.ParsePath(r, &path); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		var req types.BackupRestoreReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		resp, err := logic.RestoreBackup(r.Context(), svcCtx, path.Filename, req.Confirm)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		common.Ok(w, resp)
	}
}

// uploadHandler 处理 multipart/form-data 上传
// 表单字段名固定为 "file"
func uploadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims := accessClaims(r, svcCtx.Config.AuthSecret)
		if claims == nil || (claims.UserType != "user" && claims.UserType != "admin" && claims.UserType != "master") {
			common.JsonError(w, common.ErrRoleForbidden)
			return
		}
		// Enforce a real request body limit; ParseMultipartForm only limits memory use.
		r.Body = http.MaxBytesReader(w, r.Body, 32<<20)
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			common.JsonError(w, common.NewBizError(7002, "文件解析失败，最大 32MB"))
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			common.JsonError(w, common.ErrParamMissing)
			return
		}
		defer file.Close()

		l := logic.NewUploadLogic(r.Context(), svcCtx)
		resp, err := l.UploadFromReader(header.Filename, header.Header.Get("Content-Type"), header.Size, file)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

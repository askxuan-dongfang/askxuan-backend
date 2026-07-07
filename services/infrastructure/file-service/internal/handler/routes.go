package handler

import (
	"net/http"

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
			Method:  http.MethodGet,
			Path:    "/api/v1/files/presigned",
			Handler: presignedHandler(svcCtx),
		},
		{
			Method:  http.MethodPost,
			Path:    "/api/v1/files/upload",
			Handler: uploadHandler(svcCtx),
		},
	})
}

func presignedHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.PresignReq
		if err := httpx.Parse(r, &req); err != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		l := logic.NewPresignLogic(r.Context(), svcCtx)
		resp, err := l.Presigned(&req)
		if err != nil {
			common.JsonError(w, err)
		} else {
			common.Ok(w, resp)
		}
	}
}

// uploadHandler 处理 multipart/form-data 上传
// 表单字段名固定为 "file"
func uploadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 限制上传大小 32MB
		if err := r.ParseMultipartForm(32 << 20); err != nil {
			common.JsonError(w, common.NewBizError(7002, "文件解析失败，最大 32MB"))
			return
		}
		file, header, err := r.FormFile("file")
		if err != nil {
			common.JsonError(w, common.NewBizError(7003, "缺少 file 字段"))
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

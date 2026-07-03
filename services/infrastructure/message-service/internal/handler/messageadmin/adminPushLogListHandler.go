// Code scaffolded by goctl. Safe to edit.

package messageadmin

import (
	"net/http"

	"github.com/askxuan/message-service/internal/logic/messageadmin"
	"github.com/askxuan/message-service/internal/svc"
	"github.com/askxuan/message-service/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func AdminPushLogListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.PushLogListReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := messageadmin.NewAdminPushLogListLogic(r.Context(), svcCtx)
		resp, err := l.AdminPushLogList(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}

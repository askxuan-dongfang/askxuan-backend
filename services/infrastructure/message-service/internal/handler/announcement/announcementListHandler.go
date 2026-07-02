// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package announcement

import (
	"net/http"

	"github.com/askxuan/message-service/internal/logic/announcement"
	"github.com/askxuan/message-service/internal/svc"
	"github.com/askxuan/message-service/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func AnnouncementListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AnnouncementListReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := announcement.NewAnnouncementListLogic(r.Context(), svcCtx)
		resp, err := l.AnnouncementList(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}

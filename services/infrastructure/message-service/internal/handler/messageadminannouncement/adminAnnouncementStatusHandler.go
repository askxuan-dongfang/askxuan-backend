// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package messageadminannouncement

import (
	"net/http"

	"github.com/askxuan/message-service/internal/logic/messageadminannouncement"
	"github.com/askxuan/message-service/internal/svc"
	"github.com/askxuan/message-service/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func AdminAnnouncementStatusHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AnnouncementStatusReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := messageadminannouncement.NewAdminAnnouncementStatusLogic(r.Context(), svcCtx)
		resp, err := l.AdminAnnouncementStatus(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}

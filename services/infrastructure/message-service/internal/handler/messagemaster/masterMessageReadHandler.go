// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package messagemaster

import (
	"net/http"

	"github.com/askxuan/message-service/internal/logic/messagemaster"
	"github.com/askxuan/message-service/internal/svc"
	"github.com/askxuan/message-service/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

// MasterMessageReadHandler 法师标记消息已读
func MasterMessageReadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.MasterMessageReadReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := messagemaster.NewMasterMessageReadLogic(r.Context(), svcCtx)
		resp, err := l.MasterMessageRead(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}

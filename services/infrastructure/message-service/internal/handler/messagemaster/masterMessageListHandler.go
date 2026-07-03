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

// MasterMessageListHandler 法师消息列表
func MasterMessageListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.MasterMessageListReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := messagemaster.NewMasterMessageListLogic(r.Context(), svcCtx)
		resp, err := l.MasterMessageList(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}

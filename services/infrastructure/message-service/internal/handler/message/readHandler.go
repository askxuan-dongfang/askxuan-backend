// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package message

import (
	"net/http"

	messageLogic "github.com/askxuan/message-service/internal/logic/message"
	"github.com/askxuan/message-service/internal/svc"
	"github.com/askxuan/message-service/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func ReadHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ReadReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		logic := messageLogic.NewReadLogic(r.Context(), svcCtx)
		resp, err := logic.Read(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}
		httpx.OkJsonCtx(r.Context(), w, resp)
	}
}

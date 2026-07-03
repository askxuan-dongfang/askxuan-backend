// Code scaffolded by goctl. Safe to edit.

package messagecustomer

import (
	"net/http"

	"github.com/askxuan/message-service/internal/logic/messagecustomer"
	"github.com/askxuan/message-service/internal/svc"
	"github.com/askxuan/message-service/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func RegisterDeviceTokenHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.DeviceTokenRegisterReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := messagecustomer.NewRegisterDeviceTokenLogic(r.Context(), svcCtx)
		resp, err := l.RegisterDeviceToken(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}

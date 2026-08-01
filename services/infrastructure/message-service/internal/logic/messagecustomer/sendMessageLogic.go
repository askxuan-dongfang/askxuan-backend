// Code scaffolded by goctl. Safe to edit.

package messagecustomer

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/message-service/internal/svc"
	"github.com/askxuan/message-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type SendMessageLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSendMessageLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SendMessageLogic {
	return &SendMessageLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *SendMessageLogic) SendMessage(req *types.SendMessageReq) (*types.SendMessageResp, error) {
	// Kept only for old app versions. Consultation now requires a paid booking and
	// must use booking-service so the entitlement is checked server-side.
	return nil, common.ErrBookingChatUnavailable
}

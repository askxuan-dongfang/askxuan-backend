// Code scaffolded by goctl. Safe to edit.

package messagecustomer

import (
	"context"
	"strings"

	"github.com/askxuan/common"
	"github.com/askxuan/message-service/internal/svc"
	"github.com/askxuan/message-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UnbindDeviceTokenLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUnbindDeviceTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnbindDeviceTokenLogic {
	return &UnbindDeviceTokenLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *UnbindDeviceTokenLogic) UnbindDeviceToken(req *types.DeviceTokenUnbindReq) (*types.DeviceTokenResp, error) {
	userId := strings.TrimSpace(req.UserId)
	token := strings.TrimSpace(req.DeviceToken)
	if userId == "" || token == "" {
		return nil, common.ErrParamMissing
	}
	_, err := l.svcCtx.DeviceTokenModel.Deactivate(l.ctx, userId, token)
	if err != nil {
		l.Errorf("解绑 device token 失败: %v", err)
		return nil, common.ErrSystem
	}
	return &types.DeviceTokenResp{Id: 0, Status: "inactive"}, nil
}

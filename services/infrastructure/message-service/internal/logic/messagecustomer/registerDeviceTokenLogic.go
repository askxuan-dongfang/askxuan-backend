// Code scaffolded by goctl. Safe to edit.

package messagecustomer

import (
	"context"
	"strings"

	"github.com/askxuan/common"
	"github.com/askxuan/message-service/internal/model"
	"github.com/askxuan/message-service/internal/svc"
	"github.com/askxuan/message-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type RegisterDeviceTokenLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRegisterDeviceTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterDeviceTokenLogic {
	return &RegisterDeviceTokenLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *RegisterDeviceTokenLogic) RegisterDeviceToken(req *types.DeviceTokenRegisterReq) (*types.DeviceTokenResp, error) {
	userId := strings.TrimSpace(req.UserId)
	token := strings.TrimSpace(req.DeviceToken)
	if userId == "" || token == "" {
		return nil, common.ErrParamMissing
	}
	clientType := strings.TrimSpace(req.ClientType)
	if clientType == "" {
		clientType = "customer"
	}
	platform := strings.TrimSpace(req.Platform)
	if platform == "" {
		platform = "ios"
	}

	id, err := l.svcCtx.DeviceTokenModel.Upsert(l.ctx, &model.DeviceToken{
		UserId:      userId,
		ClientType:  clientType,
		Platform:    platform,
		DeviceToken: token,
		BundleId:    req.BundleId,
		AppVersion:  req.AppVersion,
	})
	if err != nil {
		l.Errorf("注册 device token 失败: %v", err)
		return nil, common.ErrSystem
	}
	return &types.DeviceTokenResp{Id: id, Status: "active"}, nil
}

// Code scaffolded by goctl. Safe to edit.

package messagecustomer

import (
	"context"
	"encoding/hex"
	"github.com/askxuan/common/middleware"
	"strconv"
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
	user := middleware.UserIDFromCtx(l.ctx)
	if user <= 0 {
		return nil, common.ErrForbidden
	}
	userId := strconv.FormatInt(user, 10)
	clientType := "customer"
	identity := "u_" + userId
	bundle := "com.dongfang.customer"
	if master := middleware.MasterIDFromCtx(l.ctx); master > 0 {
		clientType = "master"
		identity = "m_" + strconv.FormatInt(master, 10)
		bundle = "com.askxuan.master"
	}
	token := strings.ToLower(strings.TrimSpace(req.DeviceToken))
	decoded, err := hex.DecodeString(token)
	if err != nil || len(decoded) < 16 || len(decoded) > 127 {
		return nil, common.NewBizError(40032, "需要真实 APNs 设备令牌")
	}
	if req.ClientType != "" && req.ClientType != clientType {
		return nil, common.ErrForbidden
	}
	if req.BundleId != bundle {
		return nil, common.ErrForbidden
	}
	environment := req.Environment
	if environment == "" {
		environment = "production"
	}
	if environment != "production" && environment != "sandbox" {
		return nil, common.ErrParamInvalid
	}
	platform := "ios"

	id, err := l.svcCtx.DeviceTokenModel.Upsert(l.ctx, &model.DeviceToken{
		UserId:       userId,
		ChatIdentity: identity, Environment: environment,
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

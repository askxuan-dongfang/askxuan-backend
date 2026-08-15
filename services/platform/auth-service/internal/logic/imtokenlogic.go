package logic

import (
	"context"
	"time"

	"github.com/askxuan/auth-service/internal/svc"
	"github.com/askxuan/auth-service/internal/types"
	"github.com/askxuan/common"

	"github.com/zeromicro/go-zero/core/logx"
)

// ImTokenLogic 为已登录用户重新签发 OpenIM 登录 token。
// openimUserID 由网关注入的身份头推导：C 端 "u_"+userId，法师端 "m_"+masterId。
type ImTokenLogic struct {
	logx.Logger
	ctx          context.Context
	svcCtx       *svc.ServiceContext
	openimUserID string
}

func NewImTokenLogic(ctx context.Context, svcCtx *svc.ServiceContext, openimUserID string) *ImTokenLogic {
	return &ImTokenLogic{
		Logger:       logx.WithContext(ctx),
		ctx:          ctx,
		svcCtx:       svcCtx,
		openimUserID: openimUserID,
	}
}

func (l *ImTokenLogic) ImToken(req *types.IMTokenReq) (*types.IMTokenResp, error) {
	if l.openimUserID == "" {
		return nil, common.ErrUnauthorized
	}
	if l.svcCtx.IMClient == nil {
		l.Errorf("IMClient 未初始化（配置缺失）")
		return nil, common.ErrSystem
	}

	// 短超时，避免 OpenIM 响应慢拖垮请求
	imCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	token, err := l.svcCtx.IMClient.GetUserToken(imCtx, l.openimUserID)
	if err != nil {
		l.Errorf("获取 OpenIM token 失败 openimUserID=%s: %v", l.openimUserID, err)
		return nil, common.ErrSystem
	}
	return &types.IMTokenResp{IMToken: token}, nil
}

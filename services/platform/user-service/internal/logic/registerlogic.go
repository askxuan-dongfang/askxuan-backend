package logic

import (
	"context"

	"regexp"

	"github.com/askxuan/common"

	"github.com/askxuan/user-service/internal/svc"
	"github.com/askxuan/user-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

var mainlandMobilePattern = regexp.MustCompile(`^1[3-9][0-9]{9}$`)

// RegisterLogic 用户注册逻辑
type RegisterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterLogic {
	return &RegisterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Register 用户注册。演示环境不接真实短信，Code 仅作为旧客户端兼容字段。
func (l *RegisterLogic) Register(req *types.RegisterReq) (*types.RegisterResp, error) {
	return nil, &common.BizError{Code: 40010, Msg: "请更新客户端，使用邮箱验证注册"}
}

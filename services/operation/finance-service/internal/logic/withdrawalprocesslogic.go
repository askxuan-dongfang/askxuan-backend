package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/finance-service/internal/svc"
	"github.com/askxuan/finance-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// WithdrawalProcessLogic 提现打款逻辑
type WithdrawalProcessLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWithdrawalProcessLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WithdrawalProcessLogic {
	return &WithdrawalProcessLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// WithdrawalProcess 发起打款（approved → processing / failed → processing）
func (l *WithdrawalProcessLogic) WithdrawalProcess(req *types.WithdrawalProcessReq) (*types.WithdrawalProcessResp, error) {
	// TODO: 校验状态流转 CanTransitWithdrawal + 调用第三方打款 + 更新状态
	return nil, common.ErrNotImplemented
}

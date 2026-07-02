package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/finance-service/internal/svc"
	"github.com/askxuan/finance-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// WithdrawalListLogic 提现列表查询逻辑
type WithdrawalListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWithdrawalListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WithdrawalListLogic {
	return &WithdrawalListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// WithdrawalList 提现申请列表，支持按 applicantType/status 筛选 + 分页
func (l *WithdrawalListLogic) WithdrawalList(req *types.WithdrawalListReq) (*types.WithdrawalListResp, error) {
	// TODO: 调用 model.ListWithdrawals 查询
	return nil, common.ErrNotImplemented
}

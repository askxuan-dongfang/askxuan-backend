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

// WithdrawalProcess keeps the existing API closed until a real payout channel is
// implemented. Reject before accessing storage so no request can mark an unpaid
// withdrawal as processing or successful.
func (l *WithdrawalProcessLogic) WithdrawalProcess(req *types.WithdrawalProcessReq) (*types.WithdrawalProcessResp, error) {
	if req == nil || req.Id <= 0 {
		return nil, common.ErrParam
	}
	return nil, common.NewBizError(common.ErrNotImplemented.Code, "真实打款通道尚未接入，未发起打款，提现状态未改变")
}

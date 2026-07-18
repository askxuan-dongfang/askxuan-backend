package logic

import (
	"context"
	"time"

	"github.com/askxuan/common"
	"github.com/askxuan/finance-service/internal/model"
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
	w, ok := model.FindWithdrawalByID(req.Id)
	if !ok {
		return nil, common.NewBizError(40404, "提现单不存在")
	}
	if !model.CanTransitWithdrawal(w.Status, model.WithdrawalProcessing) {
		return nil, common.ErrStatusInvalid
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	// approved → processing
	if !model.UpdateWithdrawalStatus(req.Id, model.WithdrawalProcessing, "", now) {
		return nil, common.ErrStatusInvalid
	}
	// mock 打款成功 → processing → success
	if !model.UpdateWithdrawalStatus(req.Id, model.WithdrawalSuccess, "", now) {
		return nil, common.ErrStatusInvalid
	}
	return &types.WithdrawalProcessResp{Id: req.Id, Status: model.WithdrawalSuccess}, nil
}

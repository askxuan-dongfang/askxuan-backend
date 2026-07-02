package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/finance-service/internal/svc"
	"github.com/askxuan/finance-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// WithdrawalAuditLogic 提现审核逻辑
type WithdrawalAuditLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWithdrawalAuditLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WithdrawalAuditLogic {
	return &WithdrawalAuditLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// WithdrawalAudit 审核提现（approve → approved / reject → rejected）
// 参照 state-machines.md 提现状态机
func (l *WithdrawalAuditLogic) WithdrawalAudit(req *types.WithdrawalAuditReq) (*types.WithdrawalAuditResp, error) {
	// TODO: 校验 action + 状态流转 CanTransitWithdrawal + 更新状态 + MQ通知
	return nil, common.ErrNotImplemented
}

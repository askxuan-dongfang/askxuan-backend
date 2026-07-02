package logic

import (
	"context"
	"fmt"
	"time"

	"github.com/askxuan/common"
	"github.com/askxuan/finance-service/internal/model"
	"github.com/askxuan/finance-service/internal/mq"
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
	var targetStatus string
	switch req.Action {
	case "approve":
		targetStatus = model.WithdrawalApproved
	case "reject":
		targetStatus = model.WithdrawalRejected
	default:
		return nil, common.ErrParam
	}
	w, ok := model.FindWithdrawalByID(req.Id)
	if !ok {
		return nil, common.NewBizError(40404, "提现单不存在")
	}
	if !model.CanTransitWithdrawal(w.Status, targetStatus) {
		return nil, common.ErrStatusInvalid
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	model.UpdateWithdrawalStatus(req.Id, targetStatus, now, "")
	// 发 MQ 通知
	_ = l.svcCtx.MqProducer.PublishWithdrawalNotify(l.ctx, mq.WithdrawalNotify{
		WithdrawalId: fmt.Sprintf("%d", req.Id),
		UserId:       w.ApplicantId,
		Amount:       w.Amount,
		Status:       targetStatus,
		Time:         now,
	})
	return &types.WithdrawalAuditResp{Id: req.Id, Status: targetStatus}, nil
}

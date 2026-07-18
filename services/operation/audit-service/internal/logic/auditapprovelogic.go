package logic

import (
	"context"
	"fmt"
	"time"

	"github.com/askxuan/audit-service/internal/model"
	"github.com/askxuan/audit-service/internal/mq"
	"github.com/askxuan/audit-service/internal/svc"
	"github.com/askxuan/audit-service/internal/types"
	"github.com/askxuan/common"

	"github.com/zeromicro/go-zero/core/logx"
)

// AuditApproveLogic 审核通过逻辑
type AuditApproveLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAuditApproveLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AuditApproveLogic {
	return &AuditApproveLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// AuditApprove 审核通过
// 参照 state-machines.md 7.1/7.2/7.3 审核状态机
func (l *AuditApproveLogic) AuditApprove(req *types.AuditApproveReq) (*types.AuditApproveResp, error) {
	// 查询审核记录
	a, ok := model.FindAuditQueueByID(req.Id)
	if !ok {
		return nil, common.NewBizError(40404, "审核记录不存在")
	}
	// 校验状态流转
	if !model.CanTransitAudit(a.Status, model.AuditStatusApproved) {
		return nil, common.ErrStatusInvalid
	}
	// 更新审核状态
	now := time.Now().Format("2006-01-02 15:04:05")
	if !model.TransitionAuditQueue(req.Id, model.AuditStatusApproved, req.AuditorId, now, req.Remark, "approve") {
		return nil, common.ErrStatusInvalid
	}
	// 发 MQ 通知
	_ = l.svcCtx.MqProducer.PublishAuditResult(l.ctx, mq.AuditResult{
		AuditId: fmt.Sprintf("%d", req.Id),
		BizType: a.BizType,
		BizId:   a.BizId,
		Result:  "approved",
		Time:    now,
	})
	return &types.AuditApproveResp{Id: req.Id, Status: model.AuditStatusApproved}, nil
}

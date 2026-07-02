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

// AuditRejectLogic 审核驳回逻辑
type AuditRejectLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAuditRejectLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AuditRejectLogic {
	return &AuditRejectLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// AuditReject 审核驳回（需填remark）
func (l *AuditRejectLogic) AuditReject(req *types.AuditRejectReq) (*types.AuditRejectResp, error) {
	// 查询审核记录
	a, ok := model.FindAuditQueueByID(req.Id)
	if !ok {
		return nil, common.NewBizError(40404, "审核记录不存在")
	}
	// 校验状态流转
	if !model.CanTransitAudit(a.Status, model.AuditStatusRejected) {
		return nil, common.ErrStatusInvalid
	}
	// 更新审核状态
	now := time.Now().Format("2006-01-02 15:04:05")
	model.UpdateAuditQueueStatus(req.Id, model.AuditStatusRejected, req.AuditorId, now, req.Remark)
	// 写入审核日志
	model.InsertAuditLog(model.AuditLog{
		AuditId:    req.Id,
		Action:     "reject",
		OperatorId: req.AuditorId,
		Remark:     req.Remark,
	})
	// 发 MQ 通知
	_ = l.svcCtx.MqProducer.PublishAuditResult(l.ctx, mq.AuditResult{
		AuditId: fmt.Sprintf("%d", req.Id),
		BizType: a.BizType,
		BizId:   a.BizId,
		Result:  "rejected",
		Time:    now,
	})
	return &types.AuditRejectResp{Id: req.Id, Status: model.AuditStatusRejected}, nil
}

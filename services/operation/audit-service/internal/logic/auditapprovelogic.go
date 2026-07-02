package logic

import (
	"context"

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
	// TODO: 校验状态流转 CanTransitAudit + 更新状态 + 写入 audit_log + MQ通知
	return nil, common.ErrNotImplemented
}

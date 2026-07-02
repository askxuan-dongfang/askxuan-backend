package logic

import (
	"context"

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
	// TODO: 校验 remark 非空 + 状态流转 + 更新状态 + 写入 audit_log + MQ通知
	return nil, common.ErrNotImplemented
}

package logic

import (
	"context"

	"github.com/askxuan/audit-service/internal/svc"
	"github.com/askxuan/audit-service/internal/types"
	"github.com/askxuan/common"

	"github.com/zeromicro/go-zero/core/logx"
)

// AuditQueueDetailLogic 审核详情逻辑
type AuditQueueDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAuditQueueDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AuditQueueDetailLogic {
	return &AuditQueueDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// AuditQueueDetail 按ID查询审核详情
func (l *AuditQueueDetailLogic) AuditQueueDetail(req *types.AuditQueueDetailReq) (*types.AuditQueue, error) {
	// TODO: 调用 model.FindAuditQueueByID 查询
	return nil, common.ErrNotImplemented
}

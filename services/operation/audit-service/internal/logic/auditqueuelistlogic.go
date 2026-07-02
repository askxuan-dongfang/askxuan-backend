package logic

import (
	"context"

	"github.com/askxuan/audit-service/internal/svc"
	"github.com/askxuan/audit-service/internal/types"
	"github.com/askxuan/common"

	"github.com/zeromicro/go-zero/core/logx"
)

// AuditQueueListLogic 审核队列列表逻辑
type AuditQueueListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAuditQueueListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AuditQueueListLogic {
	return &AuditQueueListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// AuditQueueList 审核队列，支持按 bizType/status 筛选 + 分页
func (l *AuditQueueListLogic) AuditQueueList(req *types.AuditQueueListReq) (*types.AuditQueueListResp, error) {
	// TODO: 调用 model.ListAuditQueue 查询
	return nil, common.ErrNotImplemented
}

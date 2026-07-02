package logic

import (
	"context"

	"github.com/askxuan/audit-service/internal/model"
	"github.com/askxuan/audit-service/internal/svc"
	"github.com/askxuan/audit-service/internal/types"

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
	list, total := model.ListAuditQueue(req.BizType, req.Status, req.Page, req.Size)
	// 转换为 []types.AuditQueue
	out := make([]types.AuditQueue, 0, len(list))
	for _, a := range list {
		out = append(out, types.AuditQueue{
			Id:              a.Id,
			BizType:         a.BizType,
			BizId:           a.BizId,
			SubmitterId:     a.SubmitterId,
			ContentSnapshot: a.ContentSnapshot,
			Status:          a.Status,
			AuditorId:       a.AuditorId,
			AuditTime:       a.AuditTime,
			AuditRemark:     a.AuditRemark,
			CreateTime:      a.CreateTime,
		})
	}
	return &types.AuditQueueListResp{
		Total: total,
		List:  out,
		Page:  req.Page,
		Size:  req.Size,
	}, nil
}

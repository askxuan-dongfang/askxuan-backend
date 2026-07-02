package logic

import (
	"context"

	"github.com/askxuan/audit-service/internal/model"
	"github.com/askxuan/audit-service/internal/svc"
	"github.com/askxuan/audit-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// StatisticsLogic 审核统计逻辑
type StatisticsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewStatisticsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *StatisticsLogic {
	return &StatisticsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Statistics 审核统计：总量/待审/已通过/已驳回/通过率/平均审核时长
func (l *StatisticsLogic) Statistics(req *types.StatisticsReq) (*types.StatisticsResp, error) {
	// 遍历 audit_queue store 计算统计
	list, _ := model.ListAuditQueue(req.BizType, "", 1, 100000) // 取全部
	total := int64(len(list))
	var pending, approved, rejected int64
	for _, a := range list {
		switch a.Status {
		case model.AuditStatusPending:
			pending++
		case model.AuditStatusApproved:
			approved++
		case model.AuditStatusRejected:
			rejected++
		}
	}
	// 通过率 = approved / (approved + rejected) * 100
	passRate := 0.0
	if approved+rejected > 0 {
		passRate = float64(approved) / float64(approved+rejected) * 100
	}
	return &types.StatisticsResp{
		TotalCount:    total,
		PendingCount:  pending,
		ApprovedCount: approved,
		RejectedCount: rejected,
		PassRate:      passRate,
		AvgAuditTime:  0, // 简化处理
	}, nil
}

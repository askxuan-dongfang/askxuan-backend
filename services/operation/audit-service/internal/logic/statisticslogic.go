package logic

import (
	"context"

	"github.com/askxuan/audit-service/internal/svc"
	"github.com/askxuan/audit-service/internal/types"
	"github.com/askxuan/common"

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
	// TODO: 聚合 audit_queue 数据计算统计指标
	return nil, common.ErrNotImplemented
}

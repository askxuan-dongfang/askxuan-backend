package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/finance-service/internal/svc"
	"github.com/askxuan/finance-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// ReportsLogic 财务报表逻辑
type ReportsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewReportsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReportsLogic {
	return &ReportsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Reports 财务报表，支持按时间范围 + 类型筛选
func (l *ReportsLogic) Reports(req *types.ReportReq) (*types.ReportResp, error) {
	// TODO: 按时间范围聚合 settlement + withdrawal 数据
	return nil, common.ErrNotImplemented
}

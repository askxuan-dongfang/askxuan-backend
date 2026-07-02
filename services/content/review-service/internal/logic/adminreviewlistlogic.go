package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/review-service/internal/svc"
	"github.com/askxuan/review-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// AdminReviewListLogic 管理台评价列表逻辑
type AdminReviewListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminReviewListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminReviewListLogic {
	return &AdminReviewListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// AdminReviewList 管理台评价列表，支持按 status 筛选（含hidden）
func (l *AdminReviewListLogic) AdminReviewList(req *types.AdminReviewListReq) (*types.AdminReviewListResp, error) {
	// TODO: 调用 model.ListReviews 查询（含 hidden 状态）
	return nil, common.ErrNotImplemented
}

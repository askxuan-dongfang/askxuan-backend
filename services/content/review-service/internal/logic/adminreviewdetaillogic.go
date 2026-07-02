package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/review-service/internal/svc"
	"github.com/askxuan/review-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// AdminReviewDetailLogic 管理台评价详情逻辑
type AdminReviewDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminReviewDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminReviewDetailLogic {
	return &AdminReviewDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// AdminReviewDetail 管理台评价详情（含回复列表）
func (l *AdminReviewDetailLogic) AdminReviewDetail(req *types.ReviewDetailReq) (*types.Review, error) {
	// TODO: 调用 model.FindReviewByID + 查询回复列表
	return nil, common.ErrNotImplemented
}

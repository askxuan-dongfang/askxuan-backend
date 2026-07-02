package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/review-service/internal/svc"
	"github.com/askxuan/review-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// ReviewListLogic C端评价列表逻辑
type ReviewListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewReviewListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReviewListLogic {
	return &ReviewListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ReviewList C端评价列表，按 target 查询，仅返回 normal 状态
func (l *ReviewListLogic) ReviewList(req *types.ReviewListReq) (*types.ReviewListResp, error) {
	// TODO: 调用 model.ListReviews 查询 status=normal 的评价
	return nil, common.ErrNotImplemented
}

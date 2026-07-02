package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/review-service/internal/svc"
	"github.com/askxuan/review-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// ReviewDetailLogic 评价详情逻辑
type ReviewDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewReviewDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReviewDetailLogic {
	return &ReviewDetailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ReviewDetail 按ID查询评价详情
func (l *ReviewDetailLogic) ReviewDetail(req *types.ReviewDetailReq) (*types.Review, error) {
	// TODO: 调用 model.FindReviewByID 查询
	return nil, common.ErrNotImplemented
}

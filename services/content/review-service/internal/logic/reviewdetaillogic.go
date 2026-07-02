package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/review-service/internal/model"
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
	r, ok := model.FindReviewByID(req.Id)
	if !ok {
		return nil, common.ErrReviewNotFound
	}
	return &types.Review{
		Id:         r.Id,
		ReviewNo:   r.ReviewNo,
		UserId:     r.UserId,
		TargetType: r.TargetType,
		TargetId:   r.TargetId,
		Rating:     r.Rating,
		Content:    r.Content,
		Images:     r.Images,
		Status:     r.Status,
		CreateTime: r.CreateTime,
	}, nil
}

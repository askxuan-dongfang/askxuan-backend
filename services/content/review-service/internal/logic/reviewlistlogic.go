package logic

import (
	"context"

	"github.com/askxuan/review-service/internal/model"
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
	list, total := model.ListReviews(req.TargetType, req.TargetId, req.UserId, req.Rating, model.ReviewStatusNormal, req.Page, req.Size)

	result := make([]types.Review, 0, len(list))
	for _, r := range list {
		result = append(result, types.Review{
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
		})
	}

	return &types.ReviewListResp{
		Total: total,
		List:  result,
		Page:  req.Page,
		Size:  req.Size,
	}, nil
}

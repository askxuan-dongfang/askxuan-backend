package logic

import (
	"context"
	"strconv"

	"github.com/askxuan/common"
	"github.com/askxuan/review-service/internal/model"
	"github.com/askxuan/review-service/internal/mq"
	"github.com/askxuan/review-service/internal/svc"
	"github.com/askxuan/review-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// CreateReviewLogic 提交评价逻辑
type CreateReviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateReviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateReviewLogic {
	return &CreateReviewLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// CreateReview 用户提交评价
// 1.校验评分 2.写入review记录 3.发送MQ通知
func (l *CreateReviewLogic) CreateReview(req *types.CreateReviewReq) (*types.CreateReviewResp, error) {
	// 校验评分 1-5
	if req.Rating < 1 || req.Rating > 5 {
		return nil, common.ErrParam
	}

	// 写入评价记录
	review := model.CreateReview(model.Review{
		UserId:     req.UserId,
		TargetType: req.TargetType,
		TargetId:   req.TargetId,
		Rating:     req.Rating,
		Content:    req.Content,
		Images:     req.Images,
		Status:     model.ReviewStatusNormal,
	})

	// 发送 MQ 通知（action=created）
	_ = l.svcCtx.MqProducer.PublishReviewNotify(l.ctx, mq.ReviewNotify{
		ReviewId:   strconv.FormatInt(review.Id, 10),
		UserId:     review.UserId,
		TargetType: review.TargetType,
		TargetId:   review.TargetId,
		Action:     "created",
	})

	return &types.CreateReviewResp{
		Id:     review.Id,
		Status: review.Status,
	}, nil
}

package logic

import (
	"context"

	"github.com/askxuan/common"
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
// 1.校验订单可评价 2.写入review记录 3.发送MQ通知
func (l *CreateReviewLogic) CreateReview(req *types.CreateReviewReq) (*types.CreateReviewResp, error) {
	// TODO: 校验 rating 1-5 + 写入 review + MQ review.notify
	return nil, common.ErrNotImplemented
}

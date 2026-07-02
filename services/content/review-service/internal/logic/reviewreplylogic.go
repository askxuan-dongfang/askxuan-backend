package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/review-service/internal/svc"
	"github.com/askxuan/review-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// ReviewReplyLogic 回复评价逻辑
type ReviewReplyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewReviewReplyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReviewReplyLogic {
	return &ReviewReplyLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ReviewReply 寺院管理员/法师/平台回复评价
func (l *ReviewReplyLogic) ReviewReply(req *types.ReviewReplyReq) (*types.ReviewReplyResp, error) {
	// TODO: 写入 review_reply 记录
	return nil, common.ErrNotImplemented
}

package logic

import (
	"context"
	"github.com/askxuan/common/middleware"
	"net/url"

	"github.com/askxuan/common"
	"github.com/askxuan/review-service/internal/model"
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
	r, err := model.FindReviewByID(l.ctx, req.Id)
	if err != nil {
		return nil, common.ErrReviewNotFound
	}
	if err = ownsReview(l.ctx, r); err != nil {
		return nil, err
	}
	if r.TargetType != model.TargetTypeBooking {
		return nil, common.ErrForbidden
	}
	prefix := "/admin/bookings/"
	if middleware.MasterIDFromCtx(l.ctx) > 0 {
		prefix = "/admin/masters/bookings/"
	}
	if err = upstream(l.ctx, "PUT", prefix+url.PathEscape(r.TargetId)+"/review/reply", map[string]string{"masterReply": req.Content}, nil); err != nil {
		return nil, err
	}
	return &types.ReviewReplyResp{Id: r.Id}, nil
}

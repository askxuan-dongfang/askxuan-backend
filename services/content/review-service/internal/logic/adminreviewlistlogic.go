package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/review-service/internal/model"
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
	ctx, code, err := providerScope(l.ctx)
	if err != nil {
		return nil, err
	}
	list, total, err := model.ListReviews(ctx, req.TargetType, req.TargetId, "", req.Rating, req.Status, code, req.Page, req.Size)
	if err != nil {
		l.Errorf("查询评价列表失败: %v", err)
		return nil, common.ErrSystem
	}

	result := make([]types.Review, 0, len(list))
	for _, r := range list {
		result = append(result, types.Review{
			Id:         r.Id,
			ReviewNo:   r.ReviewNo,
			UserId:     r.UserId,
			TargetType: r.TargetType,
			TargetId:   r.TargetId,
			MasterCode: r.MasterCode,
			Rating:     r.Rating,
			Content:    r.Content,
			Images:     r.Images,
			Status:     r.Status,
			CreateTime: r.CreateTime, TempleCode: r.TempleCode, TempleName: r.TempleName, MasterName: r.MasterName, ServiceName: r.ServiceName, MasterReply: r.MasterReply,
		})
	}

	return &types.AdminReviewListResp{
		Total: total,
		List:  result,
		Page:  req.Page,
		Size:  req.Size,
	}, nil
}

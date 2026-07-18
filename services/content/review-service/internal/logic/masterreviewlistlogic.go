package logic

import (
	"context"
	"fmt"

	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/review-service/internal/model"
	"github.com/askxuan/review-service/internal/svc"
	"github.com/askxuan/review-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// MasterReviewListLogic 法师台评价列表逻辑
type MasterReviewListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMasterReviewListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MasterReviewListLogic {
	return &MasterReviewListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// MasterReviewList 法师查询自己的评价列表（masterId 从 JWT 获取，转为 masterCode 过滤）
func (l *MasterReviewListLogic) MasterReviewList(req *types.MasterReviewListReq) (*types.MasterReviewListResp, error) {
	masterID := middleware.MasterIDFromCtx(l.ctx)
	if masterID == 0 {
		return nil, common.ErrUnauthorized
	}

	// masterId (int64) 转为 masterCode (如 M001)
	masterCode := fmt.Sprintf("M%03d", masterID)

	list, total, err := model.ListReviews(l.ctx, "", "", "", req.Rating, model.ReviewStatusNormal, masterCode, req.Page, req.Size)
	if err != nil {
		l.Errorf("查询法师评价失败: %v", err)
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
			CreateTime: r.CreateTime,
		})
	}

	return &types.MasterReviewListResp{
		Total: total,
		List:  result,
		Page:  req.Page,
		Size:  req.Size,
	}, nil
}

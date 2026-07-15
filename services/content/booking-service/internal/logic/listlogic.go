package logic

import (
	"context"

	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/booking-service/internal/types"
	"github.com/askxuan/common"

	"github.com/zeromicro/go-zero/core/logx"
)

// ListLogic 预约列表查询逻辑
type ListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListLogic {
	return &ListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// List 预约列表查询，支持按 userId/status/templeId 筛选 + 分页
func (l *ListLogic) List(req *types.ListReq) (*types.ListResp, error) {
	userID, err := authenticatedUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	page := req.Page
	size := req.Size
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	list, total, err := l.svcCtx.BookingModel.FindList(l.ctx, userID, req.Status, req.TempleId, page, size)
	if err != nil {
		l.Errorf("查询预约列表失败: %v", err)
		return nil, common.ErrSystem
	}

	out := make([]types.Booking, 0, len(list))
	for _, b := range list {
		out = append(out, types.Booking(*b))
	}
	return &types.ListResp{
		Total: total,
		List:  out,
		Page:  page,
		Size:  size,
	}, nil
}

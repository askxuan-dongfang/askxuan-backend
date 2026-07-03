package logic

import (
	"context"
	"errors"

	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/booking-service/internal/types"
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ============ 法师工作台 - 预约 Logic ============

// MasterBookingListLogic 法师预约列表
type MasterBookingListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMasterBookingListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MasterBookingListLogic {
	return &MasterBookingListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// MasterBookingList 法师查询自己的预约列表（masterId 从 JWT 获取）
func (l *MasterBookingListLogic) MasterBookingList(req *types.MasterBookingListReq) (*types.MasterBookingListResp, error) {
	masterID := middleware.MasterIDFromCtx(l.ctx)
	if masterID == 0 {
		return nil, common.ErrUnauthorized
	}

	// 通过 masterId 查询 master_code（booking 表按 master_code 关联）
	master, err := l.svcCtx.MasterReadonlyModel.FindByID(l.ctx, masterID)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrMasterNotFound
		}
		l.Errorf("法师工作台查询法师信息失败: %v", err)
		return nil, common.ErrSystem
	}
	masterCode := master.Code

	page := req.Page
	size := req.Size
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	// 复用 FindAdminList，传空 templeId + masterCode 作为过滤条件
	list, total, err := l.svcCtx.BookingModel.FindAdminList(l.ctx, "", req.Status, masterCode, page, size)
	if err != nil {
		l.Errorf("法师工作台查询预约列表失败: %v", err)
		return nil, common.ErrSystem
	}

	out := make([]types.Booking, 0, len(list))
	for _, b := range list {
		out = append(out, types.Booking(*b))
	}
	return &types.MasterBookingListResp{
		Total: total,
		List:  out,
		Page:  page,
		Size:  size,
	}, nil
}

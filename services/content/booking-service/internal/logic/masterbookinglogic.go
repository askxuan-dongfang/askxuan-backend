package logic

import (
	"context"
	"errors"

	"github.com/askxuan/booking-service/internal/model"
	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/booking-service/internal/types"
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

	// 通过 gRPC 查询 master_code（booking 表按不可变快照关联）
	master, err := l.svcCtx.MasterClient.GetByID(l.ctx, masterID)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, common.ErrMasterNotFound
		}
		l.Errorf("法师工作台查询法师信息失败: %v", err)
		return nil, common.ErrDependencyUnavailable
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

// ============ 法师工作台 - 预约详情/确认/完成 ============

// MasterBookingDetailLogic 法师查询预约详情（仅可查看分配给自己的预约）
type MasterBookingDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMasterBookingDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MasterBookingDetailLogic {
	return &MasterBookingDetailLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *MasterBookingDetailLogic) MasterBookingDetail(req *types.DetailReq) (*types.Booking, error) {
	masterID := middleware.MasterIDFromCtx(l.ctx)
	if masterID == 0 {
		return nil, common.ErrUnauthorized
	}
	b, err := l.svcCtx.BookingModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrBookingNotFound
		}
		l.Errorf("法师查询预约详情失败: %v", err)
		return nil, common.ErrSystem
	}
	// 校验归属：仅可查看分配给自己的预约
	master, err := l.svcCtx.MasterClient.GetByID(l.ctx, masterID)
	if err != nil {
		l.Errorf("法师信息查询失败: %v", err)
		return nil, common.ErrDependencyUnavailable
	}
	if b.MasterId != master.Code {
		return nil, common.ErrForbidden
	}
	resp := types.Booking(*b)
	return &resp, nil
}

// MasterBookingConfirmLogic 法师确认预约（pending → confirmed）
type MasterBookingConfirmLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMasterBookingConfirmLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MasterBookingConfirmLogic {
	return &MasterBookingConfirmLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *MasterBookingConfirmLogic) MasterBookingConfirm(req *types.AdminBookingActionReq) (*types.StatusResp, error) {
	masterID := middleware.MasterIDFromCtx(l.ctx)
	if masterID == 0 {
		return nil, common.ErrUnauthorized
	}
	// 校验预约归属本法师
	b, err := l.svcCtx.BookingModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrBookingNotFound
		}
		return nil, common.ErrSystem
	}
	master, err := l.svcCtx.MasterClient.GetByID(l.ctx, masterID)
	if err != nil {
		return nil, common.ErrDependencyUnavailable
	}
	if b.MasterId != master.Code {
		return nil, common.ErrForbidden
	}
	operatorId := master.Code
	return transitBookingStatus(l.Logger, l.ctx, l.svcCtx, req.Id, model.StatusConfirmed, req.Remark, operatorId, model.OperatorTypeMaster, "confirmed")
}

// MasterBookingStartLogic 法师开始服务（confirmed → in_progress）
type MasterBookingStartLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMasterBookingStartLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MasterBookingStartLogic {
	return &MasterBookingStartLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *MasterBookingStartLogic) MasterBookingStart(req *types.AdminBookingActionReq) (*types.StatusResp, error) {
	masterID := middleware.MasterIDFromCtx(l.ctx)
	if masterID == 0 {
		return nil, common.ErrUnauthorized
	}
	b, err := l.svcCtx.BookingModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrBookingNotFound
		}
		return nil, common.ErrSystem
	}
	master, err := l.svcCtx.MasterClient.GetByID(l.ctx, masterID)
	if err != nil {
		return nil, common.ErrDependencyUnavailable
	}
	if b.MasterId != master.Code {
		return nil, common.ErrForbidden
	}
	return transitBookingStatus(l.Logger, l.ctx, l.svcCtx, req.Id, model.StatusInProgress, req.Remark, master.Code, model.OperatorTypeMaster, "in_progress")
}

// MasterBookingCompleteLogic 法师完成预约（in_progress → completed）
type MasterBookingCompleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMasterBookingCompleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MasterBookingCompleteLogic {
	return &MasterBookingCompleteLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *MasterBookingCompleteLogic) MasterBookingComplete(req *types.AdminBookingActionReq) (*types.StatusResp, error) {
	masterID := middleware.MasterIDFromCtx(l.ctx)
	if masterID == 0 {
		return nil, common.ErrUnauthorized
	}
	// 校验预约归属本法师
	b, err := l.svcCtx.BookingModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrBookingNotFound
		}
		return nil, common.ErrSystem
	}
	master, err := l.svcCtx.MasterClient.GetByID(l.ctx, masterID)
	if err != nil {
		return nil, common.ErrDependencyUnavailable
	}
	if b.MasterId != master.Code {
		return nil, common.ErrForbidden
	}
	operatorId := master.Code
	return transitBookingStatus(l.Logger, l.ctx, l.svcCtx, req.Id, model.StatusCompleted, req.Remark, operatorId, model.OperatorTypeMaster, "completed")
}

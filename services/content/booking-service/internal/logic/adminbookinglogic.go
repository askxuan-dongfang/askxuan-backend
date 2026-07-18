package logic

import (
	"context"
	"errors"
	"time"

	"github.com/askxuan/booking-service/internal/model"
	"github.com/askxuan/booking-service/internal/mq"
	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/booking-service/internal/types"
	"github.com/askxuan/common"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ============ 寺院管理台 - 预约管理 Logic ============

// AdminBookingListLogic 预约列表（寺院维度）
type AdminBookingListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminBookingListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminBookingListLogic {
	return &AdminBookingListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminBookingListLogic) AdminBookingList(req *types.AdminBookingListReq) (*types.AdminBookingListResp, error) {
	if req.TempleId == "" {
		return nil, common.ErrParam
	}
	page := req.Page
	size := req.Size
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	list, total, err := l.svcCtx.BookingModel.FindAdminList(l.ctx, req.TempleId, req.Status, req.MasterId, page, size)
	if err != nil {
		l.Errorf("管理台查询预约列表失败: %v", err)
		return nil, common.ErrSystem
	}

	out := make([]types.Booking, 0, len(list))
	for _, b := range list {
		out = append(out, types.Booking(*b))
	}
	return &types.AdminBookingListResp{
		Total: total,
		List:  out,
		Page:  page,
		Size:  size,
	}, nil
}

// AdminBookingDetailLogic 预约详情
type AdminBookingDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminBookingDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminBookingDetailLogic {
	return &AdminBookingDetailLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminBookingDetailLogic) AdminBookingDetail(req *types.DetailReq) (*types.Booking, error) {
	b, err := l.svcCtx.BookingModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrBookingNotFound
		}
		l.Errorf("管理台查询预约详情失败: %v", err)
		return nil, common.ErrSystem
	}
	resp := types.Booking(*b)
	return &resp, nil
}

// AdminBookingConfirmLogic 确认预约（pending → confirmed）
type AdminBookingConfirmLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminBookingConfirmLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminBookingConfirmLogic {
	return &AdminBookingConfirmLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminBookingConfirmLogic) AdminBookingConfirm(req *types.AdminBookingActionReq) (*types.StatusResp, error) {
	return transitBookingStatus(l.Logger, l.ctx, l.svcCtx, req.Id, model.StatusConfirmed, req.Remark, "admin", model.OperatorTypeTempleAdmin, "confirmed")
}

// AdminBookingCompleteLogic 完成预约（in_progress → completed）
type AdminBookingCompleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminBookingCompleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminBookingCompleteLogic {
	return &AdminBookingCompleteLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminBookingCompleteLogic) AdminBookingComplete(req *types.AdminBookingActionReq) (*types.StatusResp, error) {
	return transitBookingStatus(l.Logger, l.ctx, l.svcCtx, req.Id, model.StatusCompleted, req.Remark, "admin", model.OperatorTypeTempleAdmin, "completed")
}

// AdminBookingCancelLogic 取消预约（pending/confirmed → cancelled）
type AdminBookingCancelLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminBookingCancelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminBookingCancelLogic {
	return &AdminBookingCancelLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminBookingCancelLogic) AdminBookingCancel(req *types.AdminBookingActionReq) (*types.StatusResp, error) {
	return transitBookingStatus(l.Logger, l.ctx, l.svcCtx, req.Id, model.StatusCancelled, req.Remark, "admin", model.OperatorTypeTempleAdmin, "cancelled")
}

// AdminBookingTimeoutCancelLogic 超时取消（pending → cancelled，系统操作）
type AdminBookingTimeoutCancelLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminBookingTimeoutCancelLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminBookingTimeoutCancelLogic {
	return &AdminBookingTimeoutCancelLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminBookingTimeoutCancelLogic) AdminBookingTimeoutCancel(req *types.AdminBookingActionReq) (*types.StatusResp, error) {
	b, err := l.svcCtx.BookingModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrBookingNotFound
		}
		l.Errorf("超时取消-查询预约失败: %v", err)
		return nil, common.ErrSystem
	}

	// 校验 pending 且超过 24 小时未确认
	if b.Status != model.StatusPending {
		return nil, common.ErrBookingStatusInvalid
	}
	if b.CreatedAt != "" {
		createTime, parseErr := time.ParseInLocation("2006-01-02 15:04:05", b.CreatedAt, time.Local)
		if parseErr == nil && time.Since(createTime) < 24*time.Hour {
			return nil, common.ErrBookingStatusInvalid
		}
	}

	return transitBookingStatus(l.Logger, l.ctx, l.svcCtx, req.Id, model.StatusCancelled, "超时自动取消", "system", model.OperatorTypeSystem, "cancelled")
}

// AdminBookingStatusLogLogic 状态变更日志
type AdminBookingStatusLogLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminBookingStatusLogLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminBookingStatusLogLogic {
	return &AdminBookingStatusLogLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminBookingStatusLogLogic) AdminBookingStatusLog(req *types.StatusLogReq) (*types.StatusLogResp, error) {
	logs, err := l.svcCtx.StatusLogModel.FindByBookingId(l.ctx, req.Id)
	if err != nil {
		l.Errorf("查询状态日志失败: %v", err)
		return nil, common.ErrSystem
	}

	out := make([]types.BookingStatusLog, 0, len(logs))
	for _, log := range logs {
		out = append(out, types.BookingStatusLog(*log))
	}
	return &types.StatusLogResp{List: out}, nil
}

// AdminReviewReplyLogic 法师回复评价
type AdminReviewReplyLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminReviewReplyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminReviewReplyLogic {
	return &AdminReviewReplyLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminReviewReplyLogic) AdminReviewReply(req *types.ReviewReplyReq) (*types.BookingReview, error) {
	if req.MasterReply == "" {
		return nil, common.ErrParam
	}

	r, err := l.svcCtx.ReviewModel.UpdateReply(l.ctx, req.Id, req.MasterReply)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrReviewNotFound
		}
		l.Errorf("回复评价失败: %v", err)
		return nil, common.ErrSystem
	}
	resp := types.BookingReview(*r)
	return &resp, nil
}

// transitBookingStatus 管理台状态流转通用逻辑：校验 → 更新状态 → 记录日志 → MQ 通知
func transitBookingStatus(
	l logx.Logger,
	ctx context.Context,
	svcCtx *svc.ServiceContext,
	bookingNo, targetStatus, remark, operatorId, operatorType, action string,
) (*types.StatusResp, error) {
	// 1. 查询预约
	b, err := svcCtx.BookingModel.FindOne(ctx, bookingNo)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrBookingNotFound
		}
		l.Errorf("查询预约失败: %v", err)
		return nil, common.ErrSystem
	}

	// 2. 校验状态流转
	if !model.CanTransit(b.Status, targetStatus) {
		return nil, common.ErrBookingStatusInvalid
	}

	// 3. 更新状态
	updated, err := svcCtx.BookingModel.UpdateStatus(ctx, bookingNo, targetStatus)
	if err != nil {
		l.Errorf("更新预约状态失败: %v", err)
		return nil, common.ErrSystem
	}

	// 4. 记录状态变更日志（失败不阻断主流程）
	if logErr := svcCtx.StatusLogModel.Insert(ctx, &model.BookingStatusLog{
		BookingId:    bookingNo,
		FromStatus:   b.Status,
		ToStatus:     targetStatus,
		OperatorId:   operatorId,
		OperatorType: operatorType,
		Remark:       remark,
	}); logErr != nil {
		l.Errorf("记录状态变更日志失败: %v", logErr)
	}

	// 5. 发送 MQ 通知（失败不阻断主流程）
	if svcCtx.MqProducer != nil {
		if err := svcCtx.MqProducer.Publish(ctx, mq.BookingNotify{
			BookingId: updated.Id, UserId: updated.UserId, TempleId: updated.TempleId,
			MasterId: updated.MasterId, ServiceName: updated.ServiceName,
			BookingDate: updated.BookingDate, TotalFee: updated.TotalFee, Action: action,
		}); err != nil {
			l.Errorf("发送预约状态通知失败: %v", err)
		}
	}

	return &types.StatusResp{
		Id:     updated.Id,
		Status: updated.Status,
	}, nil
}

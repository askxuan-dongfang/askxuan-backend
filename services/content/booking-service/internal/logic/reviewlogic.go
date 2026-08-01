package logic

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/askxuan/booking-service/internal/model"
	"github.com/askxuan/booking-service/internal/mq"
	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/booking-service/internal/types"
	"github.com/askxuan/common"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ============ 预约评价 Logic ============

// CreateReviewLogic 提交评价
type CreateReviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateReviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateReviewLogic {
	return &CreateReviewLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// CreateReview 用户创建评价
// 1. 校验预约存在且为 completed 状态 2. 防重复评价 3. 落库评价 4. 状态流转 completed → reviewed 5. MQ 通知
func (l *CreateReviewLogic) CreateReview(req *types.ReviewCreateReq) (*types.ReviewCreateResp, error) {
	if req.Rating < 1 || req.Rating > 5 || req.Content == "" {
		return nil, common.ErrParam
	}

	// 1. 校验预约存在且为 completed 状态
	b, err := l.svcCtx.BookingModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrBookingNotFound
		}
		l.Errorf("查询预约失败: %v", err)
		return nil, common.ErrSystem
	}
	if b.Status != model.StatusCompleted {
		return nil, common.ErrBookingStatusInvalid
	}
	userID, authErr := authenticatedUserID(l.ctx)
	if authErr != nil {
		return nil, authErr
	}
	if b.UserId != userID {
		return nil, common.ErrForbidden
	}

	// 2. 防重复评价（uk_booking 约束保证唯一）
	existing, err := l.svcCtx.ReviewModel.FindOne(l.ctx, req.Id)
	if err == nil && existing != nil {
		return nil, common.ErrDuplicateOperation
	}
	if err != nil && !errors.Is(err, sqlx.ErrNotFound) {
		l.Errorf("查询评价失败: %v", err)
		return nil, common.ErrSystem
	}

	// 3. 落库评价
	r, err := l.svcCtx.ReviewModel.Insert(l.ctx, &model.BookingReview{
		BookingId: req.Id,
		UserId:    b.UserId,
		Rating:    req.Rating,
		Content:   req.Content,
		Images:    req.Images,
	})
	if err != nil {
		l.Errorf("创建评价失败: %v", err)
		return nil, common.ErrSystem
	}

	// 4. 状态流转 completed → reviewed
	if !model.CanTransit(b.Status, model.StatusReviewed) {
		l.Errorf("预约状态流转非法: %s → reviewed", b.Status)
	} else {
		if _, updateErr := l.svcCtx.BookingModel.UpdateStatus(l.ctx, req.Id, model.StatusReviewed); updateErr != nil {
			l.Errorf("更新预约为已评价状态失败: %v", updateErr)
		} else if logErr := l.svcCtx.StatusLogModel.Insert(l.ctx, &model.BookingStatusLog{
			BookingId:    req.Id,
			FromStatus:   model.StatusCompleted,
			ToStatus:     model.StatusReviewed,
			OperatorId:   b.UserId,
			OperatorType: model.OperatorTypeUser,
			Remark:       "用户提交评价",
		}); logErr != nil {
			l.Errorf("记录状态变更日志失败: %v", logErr)
		}
	}

	// 5. 发送 MQ 通知（失败不阻断主流程）
	if l.svcCtx.MqProducer != nil {
		images, _ := json.Marshal(req.Images)
		if err := l.svcCtx.MqProducer.Publish(l.ctx, mq.BookingNotify{
			BookingId: b.Id, UserId: b.UserId, TempleId: b.TempleId,
			TempleName: b.TempleName, MasterId: b.MasterId, MasterName: b.MasterName,
			ServiceName: b.ServiceName, BookingDate: b.BookingDate,
			ServiceFee: b.ServiceFee, MeritMoney: b.MeritMoney, TotalFee: b.TotalFee,
			Rating: req.Rating, ReviewContent: req.Content,
			ReviewImages: string(images), Action: "reviewed",
		}); err != nil {
			l.Errorf("发送评价通知失败: %v", err)
		}
	}

	return &types.ReviewCreateResp{ReviewId: r.Id}, nil
}

// ReviewDetailLogic 查看评价
type ReviewDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewReviewDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ReviewDetailLogic {
	return &ReviewDetailLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ReviewDetailLogic) ReviewDetail(req *types.ReviewDetailReq) (*types.BookingReview, error) {
	userID, authErr := authenticatedUserID(l.ctx)
	if authErr != nil {
		return nil, authErr
	}
	booking, err := l.svcCtx.BookingModel.FindOne(l.ctx, req.Id)
	if err != nil {
		return nil, common.ErrBookingNotFound
	}
	if booking.UserId != userID {
		return nil, common.ErrForbidden
	}
	r, err := l.svcCtx.ReviewModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrReviewNotFound
		}
		l.Errorf("查询评价失败: %v", err)
		return nil, common.ErrSystem
	}
	resp := types.BookingReview(*r)
	return &resp, nil
}

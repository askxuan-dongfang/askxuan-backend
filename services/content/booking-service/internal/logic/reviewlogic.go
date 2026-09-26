package logic

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/askxuan/common/mqoutbox"
	"strings"
	"time"

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
	req.Content = strings.TrimSpace(req.Content)
	if req.Rating < 1 || req.Rating > 5 || req.Content == "" || len([]rune(req.Content)) > 500 || len(req.Images) > 6 {
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

	// Persist review, state, audit and durable publication together. A failed
	// transaction leaves the completed order eligible for retry.
	images, _ := json.Marshal(req.Images)
	var reviewID int64
	err = l.svcCtx.DB.TransactCtx(l.ctx, func(ctx context.Context, tx sqlx.Session) error {
		status, err := lockBooking(ctx, tx, req.Id)
		if err != nil {
			return err
		}
		if status != model.StatusCompleted {
			return common.ErrBookingStatusInvalid
		}
		result, err := tx.ExecCtx(ctx, `INSERT INTO booking_review(booking_id,user_id,rating,content,images,master_reply,create_time) VALUES(?,?,?,?,?,'',NOW())`, req.Id, b.UserId, req.Rating, req.Content, string(images))
		if err != nil {
			return err
		}
		reviewID, err = result.LastInsertId()
		if err != nil {
			return err
		}
		if _, err = tx.ExecCtx(ctx, "UPDATE booking SET status=? WHERE booking_no=?", model.StatusReviewed, req.Id); err != nil {
			return err
		}
		if _, err = tx.ExecCtx(ctx, "INSERT INTO booking_status_log(booking_id,from_status,to_status,operator_id,operator_type,remark,create_time) VALUES(?,?,?,?,?,?,NOW())", req.Id, model.StatusCompleted, model.StatusReviewed, b.UserId, model.OperatorTypeUser, "用户提交评价"); err != nil {
			return err
		}
		body, err := json.Marshal(mq.BookingNotify{
			BookingId: b.Id, UserId: b.UserId, TempleId: b.TempleId, TempleName: b.TempleName,
			MasterId: b.MasterId, MasterName: b.MasterName, ServiceName: b.ServiceName,
			Rating: req.Rating, ReviewContent: req.Content, ReviewImages: string(images), Action: "reviewed", Time: time.Now().Format("2006-01-02 15:04:05"),
		})
		if err != nil {
			return err
		}
		return mqoutbox.Enqueue(ctx, tx, "booking:"+b.Id+":reviewed", "booking", b.Id, "booking.reviewed", mq.ExchangeBookingEvents, "", string(body))
	})
	if err != nil {
		return nil, err
	}
	return &types.ReviewCreateResp{ReviewId: reviewID}, nil
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

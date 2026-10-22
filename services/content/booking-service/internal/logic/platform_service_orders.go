package logic

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/askxuan/booking-service/internal/model"
	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/booking-service/internal/types"
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type PlatformServiceOrdersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	reader model.PlatformServiceOrderReader
}

func NewPlatformServiceOrdersLogic(ctx context.Context, s *svc.ServiceContext) *PlatformServiceOrdersLogic {
	return &PlatformServiceOrdersLogic{ctx: ctx, svcCtx: s, reader: model.NewPlatformServiceOrderReader(s.DB)}
}

// RequirePlatformServiceReader deliberately does not grant access to custom
// roles, service identities, or provider tokens. Broader grants need a separate
// contract; this API must not widen the existing provider/customer APIs.
func RequirePlatformServiceReader(ctx context.Context) error {
	kind, _ := ctx.Value(middleware.CtxKeyUserType).(string)
	if middleware.UserIDFromCtx(ctx) <= 0 || kind != "admin" {
		return common.ErrForbidden
	}
	for _, role := range middleware.RolesFromCtx(ctx) {
		if role == "platform_super" {
			return nil
		}
	}
	return common.ErrForbidden
}

func platformServiceQuery(req *types.PlatformServiceOrderListReq, consultation bool) (model.PlatformServiceOrderQuery, error) {
	q := model.PlatformServiceOrderQuery{Q: strings.TrimSpace(req.Q), Status: strings.TrimSpace(req.Status),
		PaymentStatus: strings.TrimSpace(req.PaymentStatus), TempleID: strings.TrimSpace(req.TempleID),
		MasterID: strings.TrimSpace(req.MasterID), From: strings.TrimSpace(req.From), To: strings.TrimSpace(req.To), Page: req.Page, Size: req.Size}
	if q.Page == 0 {
		q.Page = 1
	}
	if q.Size == 0 {
		q.Size = 20
	}
	if q.Page < 1 || q.Page > 1000000 || q.Size < 1 || q.Size > 100 || len([]rune(q.Q)) > 100 || len(q.TempleID) > 64 || len(q.MasterID) > 64 {
		return q, common.ErrParamInvalid
	}
	for _, date := range []string{q.From, q.To} {
		if date != "" {
			if _, err := time.Parse("2006-01-02", date); err != nil {
				return q, common.ErrParamInvalid
			}
		}
	}
	if q.From != "" && q.To != "" && q.From > q.To {
		return q, common.ErrParamInvalid
	}
	statuses := []string{model.StatusPendingPayment, model.StatusPending, model.StatusConfirmed, model.StatusInProgress, model.StatusPendingReceipt, model.StatusCompleted, model.StatusCancelled, model.StatusReviewed}
	// Refunding/refunded are persisted by the existing cancellation/outbox flow.
	payments := []string{model.PaymentStatusPending, model.PaymentStatusSuccess, model.PaymentStatusFailed, model.PaymentStatusLegacy, "refunding", "refunded"}
	if consultation {
		statuses = []string{model.ConsultationStatusPendingPayment, model.ConsultationStatusActive, model.ConsultationStatusExpired, model.ConsultationStatusClosed}
		payments = []string{model.ConsultationPaymentPending, model.ConsultationPaymentSuccess}
	}
	valid := func(value string, values []string) bool {
		if value == "" {
			return true
		}
		for _, candidate := range values {
			if value == candidate {
				return true
			}
		}
		return false
	}
	if !valid(q.Status, statuses) || !valid(q.PaymentStatus, payments) {
		return q, common.ErrParamInvalid
	}
	return q, nil
}

func (l *PlatformServiceOrdersLogic) Bookings(req *types.PlatformServiceOrderListReq) (*types.ListResp, error) {
	if err := RequirePlatformServiceReader(l.ctx); err != nil {
		return nil, err
	}
	q, err := platformServiceQuery(req, false)
	if err != nil {
		return nil, err
	}
	rows, total, err := l.reader.Bookings(l.ctx, q)
	if err != nil {
		logx.WithContext(l.ctx).Errorf("platform booking list: %v", err)
		return nil, common.ErrSystem
	}
	list := make([]types.Booking, 0, len(rows))
	for _, row := range rows {
		list = append(list, types.Booking(*row))
	}
	return &types.ListResp{List: list, Total: total, Page: q.Page, Size: q.Size}, nil
}

func (l *PlatformServiceOrdersLogic) Consultations(req *types.PlatformServiceOrderListReq) (*types.ConsultationListResp, error) {
	if err := RequirePlatformServiceReader(l.ctx); err != nil {
		return nil, err
	}
	q, err := platformServiceQuery(req, true)
	if err != nil {
		return nil, err
	}
	rows, total, err := l.reader.Consultations(l.ctx, q)
	if err != nil {
		logx.WithContext(l.ctx).Errorf("platform consultation list: %v", err)
		return nil, common.ErrSystem
	}
	list := make([]types.Consultation, 0, len(rows))
	for _, row := range rows {
		item := consultationResponse(row)
		// Order questions are available in the explicitly opened detail, not the
		// cross-provider list. Chat/message history is never queried here.
		item.Question = ""
		list = append(list, *item)
	}
	return &types.ConsultationListResp{List: list, Total: total, Page: q.Page, Size: q.Size}, nil
}

func (l *PlatformServiceOrdersLogic) Booking(id string) (*types.Booking, error) {
	if err := RequirePlatformServiceReader(l.ctx); err != nil {
		return nil, err
	}
	if strings.TrimSpace(id) == "" || len(id) > 64 {
		return nil, common.ErrParamInvalid
	}
	row, err := l.svcCtx.BookingModel.FindOne(l.ctx, id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrBookingNotFound
		}
		return nil, common.ErrSystem
	}
	response := types.Booking(*row)
	return &response, nil
}

func (l *PlatformServiceOrdersLogic) Consultation(id string) (*types.Consultation, error) {
	if err := RequirePlatformServiceReader(l.ctx); err != nil {
		return nil, err
	}
	if strings.TrimSpace(id) == "" || len(id) > 64 {
		return nil, common.ErrParamInvalid
	}
	row, err := l.svcCtx.ConsultationModel.FindOne(l.ctx, id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrConsultationNotFound
		}
		return nil, common.ErrSystem
	}
	return consultationResponse(row), nil
}

func (l *PlatformServiceOrdersLogic) Fulfillment(id string) (map[string]any, error) {
	if err := RequirePlatformServiceReader(l.ctx); err != nil {
		return nil, err
	}
	if strings.TrimSpace(id) == "" || len(id) > 64 {
		return nil, common.ErrParamInvalid
	}
	return Fulfillment(l.ctx, l.svcCtx, id)
}

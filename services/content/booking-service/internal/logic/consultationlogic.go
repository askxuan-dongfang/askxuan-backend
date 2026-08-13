package logic

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/askxuan/booking-service/internal/model"
	"github.com/askxuan/booking-service/internal/mq"
	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/booking-service/internal/types"
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ConsultationLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewConsultationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ConsultationLogic {
	return &ConsultationLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *ConsultationLogic) Quote(req *types.ConsultationQuoteReq) (*types.ConsultationQuoteResp, error) {
	if req.MasterId == "" {
		return nil, common.ErrParamMissing
	}
	master, err := l.svcCtx.MasterClient.GetByCode(l.ctx, req.MasterId)
	if err != nil {
		return nil, common.ErrMasterNotFound
	}
	enabled := master.ShelfStatus == "on_shelf" && master.PlatformStatus == "normal" && master.ConsultEnabled && master.ConsultFee > 0
	return &types.ConsultationQuoteResp{MasterId: master.Code, MasterName: master.DharmaName,
		TempleId: master.TempleCode, TempleName: master.TempleName, Enabled: enabled,
		ConsultFee: master.ConsultFee, ValidHours: int(master.ConsultValidHours),
		ResponseMinutes: int(master.ConsultResponseMinutes)}, nil
}

func (l *ConsultationLogic) Create(req *types.ConsultationCreateReq) (*types.Consultation, error) {
	userID, err := authenticatedUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	req.RequestId = strings.TrimSpace(req.RequestId)
	req.Question = strings.TrimSpace(req.Question)
	if req.RequestId == "" || req.MasterId == "" || len([]rune(req.Question)) > 500 {
		return nil, common.ErrParamInvalid
	}
	if existing, findErr := l.svcCtx.ConsultationModel.FindByRequest(l.ctx, userID, req.RequestId); findErr == nil {
		if existing.Status == model.ConsultationStatusPendingPayment {
			return l.autoPay(existing), nil
		}
		return consultationResponse(existing), nil
	}
	quote, err := l.Quote(&types.ConsultationQuoteReq{MasterId: req.MasterId})
	if err != nil {
		return nil, err
	}
	if !quote.Enabled {
		return nil, common.ErrConsultationUnavailable
	}
	snapshot, _ := json.Marshal(map[string]interface{}{"masterId": quote.MasterId, "masterName": quote.MasterName,
		"consultFee": quote.ConsultFee, "validHours": quote.ValidHours, "responseMinutes": quote.ResponseMinutes})
	created, err := l.svcCtx.ConsultationModel.Insert(l.ctx, &model.Consultation{RequestId: req.RequestId,
		UserId: userID, MasterId: quote.MasterId, MasterName: quote.MasterName, TempleId: quote.TempleId,
		TempleName: quote.TempleName, ConsultFee: quote.ConsultFee, ValidHours: quote.ValidHours,
		ResponseMinutes: quote.ResponseMinutes, Question: req.Question, PriceSnapshot: string(snapshot)})
	if err != nil {
		if existing, findErr := l.svcCtx.ConsultationModel.FindByRequest(l.ctx, userID, req.RequestId); findErr == nil {
			return consultationResponse(existing), nil
		}
		l.Errorf("创建即时咨询订单失败: %v", err)
		return nil, common.ErrSystem
	}
	return l.autoPay(created), nil
}

func (l *ConsultationLogic) Pay(req *types.ConsultationPayReq) (*types.Consultation, error) {
	row, err := l.owned(req.Id)
	if err != nil {
		return nil, err
	}
	if row.PaymentStatus == model.ConsultationPaymentSuccess {
		return consultationResponse(row), nil
	}
	if row.Status != model.ConsultationStatusPendingPayment {
		return nil, common.ErrStatusInvalid
	}
	return l.autoPay(row), nil
}

func (l *ConsultationLogic) autoPay(row *model.Consultation) *types.Consultation {
	payment, err := l.svcCtx.PaymentClient.AutoPayConsultation(l.ctx, row.Id, row.UserId, row.ConsultFee)
	if err != nil {
		l.Errorf("即时咨询模拟支付失败 consultation=%s: %v", row.Id, err)
		return consultationResponse(row)
	}
	activated, changed, err := l.svcCtx.ConsultationModel.Activate(l.ctx, row.Id, payment.PaymentNo, payment.Channel)
	if err != nil {
		l.Errorf("即时咨询支付成功后激活失败 consultation=%s: %v", row.Id, err)
		return consultationResponse(row)
	}
	if changed {
		l.publishPaid(activated)
	}
	return consultationResponse(activated)
}

func (l *ConsultationLogic) publishPaid(row *model.Consultation) {
	if l.svcCtx.MqProducer == nil || row == nil {
		return
	}
	_ = l.svcCtx.MqProducer.PublishConsultation(l.ctx, mq.ConsultationNotify{ConsultationId: row.Id,
		UserId: row.UserId, MasterId: row.MasterId, MasterName: row.MasterName, TempleId: row.TempleId,
		TempleName: row.TempleName, ConsultFee: row.ConsultFee, Action: "paid"})
}

func (l *ConsultationLogic) Detail(req *types.ConsultationDetailReq) (*types.Consultation, error) {
	row, err := l.owned(req.Id)
	if err != nil {
		return nil, err
	}
	return consultationResponse(row), nil
}

func (l *ConsultationLogic) List(req *types.ConsultationListReq) (*types.ConsultationListResp, error) {
	page, size := normalizeChatPage(req.Page, req.Size, 20)
	userID := ""
	masterCode := ""
	if masterID := middleware.MasterIDFromCtx(l.ctx); masterID > 0 {
		master, err := l.svcCtx.MasterClient.GetByID(l.ctx, masterID)
		if err != nil {
			return nil, common.ErrDependencyUnavailable
		}
		masterCode = master.Code
	} else {
		var err error
		userID, err = authenticatedUserID(l.ctx)
		if err != nil {
			return nil, err
		}
	}
	rows, total, err := l.svcCtx.ConsultationModel.List(l.ctx, userID, masterCode, req.Status, page, size)
	if err != nil {
		return nil, common.ErrSystem
	}
	list := make([]types.Consultation, 0, len(rows))
	for _, row := range rows {
		list = append(list, *consultationResponse(row))
	}
	return &types.ConsultationListResp{Total: total, List: list, Page: page, Size: size}, nil
}

func (l *ConsultationLogic) owned(id string) (*model.Consultation, error) {
	row, err := l.svcCtx.ConsultationModel.FindOne(l.ctx, id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrConsultationNotFound
		}
		return nil, common.ErrSystem
	}
	if masterID := middleware.MasterIDFromCtx(l.ctx); masterID > 0 {
		master, err := l.svcCtx.MasterClient.GetByID(l.ctx, masterID)
		if err != nil {
			return nil, common.ErrDependencyUnavailable
		}
		if master.Code != row.MasterId {
			return nil, common.ErrForbidden
		}
		return row, nil
	}
	userID, err := authenticatedUserID(l.ctx)
	if err != nil {
		return nil, err
	}
	if row.UserId != userID {
		return nil, common.ErrForbidden
	}
	return row, nil
}

func consultationResponse(row *model.Consultation) *types.Consultation {
	return &types.Consultation{Id: row.Id, UserId: row.UserId, MasterId: row.MasterId,
		MasterName: row.MasterName, TempleId: row.TempleId, TempleName: row.TempleName,
		ConsultFee: row.ConsultFee, ValidHours: row.ValidHours, ResponseMinutes: row.ResponseMinutes,
		Question: row.Question, PaymentNo: row.PaymentNo, PaymentStatus: row.PaymentStatus,
		Status: row.Status, ValidFrom: row.ValidFrom, ExpiresAt: row.ExpiresAt,
		Simulated: row.PaymentChannel == "mock", ConversationId: row.Id, CreatedAt: row.CreateTime}
}

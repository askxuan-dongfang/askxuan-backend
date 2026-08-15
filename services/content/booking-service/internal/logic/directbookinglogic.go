package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/askxuan/booking-service/internal/model"
	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/booking-service/internal/types"
	"github.com/askxuan/common"

	"github.com/zeromicro/go-zero/core/logx"
)

// masterDetail 经网关读取的大师公开详情（直约校验用）
type masterDetail struct {
	Code        string `json:"id"`
	DharmaName  string `json:"dharmaName"`
	TempleCode  string `json:"templeId"`
	TempleName  string `json:"templeName"`
	ManageBy    string `json:"manageBy"`
	ServiceTags []struct {
		ServiceCode string  `json:"serviceCode"`
		Price       float64 `json:"price"`
		Status      string  `json:"status"`
	} `json:"serviceTags"`
}

type masterDetailEnvelope struct {
	Code int          `json:"code"`
	Data masterDetail `json:"data"`
}

// DirectBookingLogic 大师直约逻辑（先付费咨询 → 再预约服务）
type DirectBookingLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDirectBookingLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DirectBookingLogic {
	return &DirectBookingLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *DirectBookingLogic) Create(masterCode string, req *types.DirectBookingReq) (*types.CreateResp, error) {
	userIDStr, err := authenticatedUserID(l.ctx)
	if err != nil {
		return nil, err
	}

	req.RequestId = strings.TrimSpace(req.RequestId)
	if masterCode == "" || req.ServiceCode == "" || req.BookingDate == "" || req.RequestId == "" {
		return nil, common.ErrParamInvalid
	}
	if _, err := time.Parse("2006-01-02", req.BookingDate); err != nil {
		return nil, common.ErrParamInvalid
	}

	// 幂等
	if existing, findErr := l.svcCtx.BookingModel.FindByRequest(l.ctx, userIDStr, req.RequestId); findErr == nil {
		if existing.PaymentStatus == model.PaymentStatusSuccess {
			return responseFromBooking(existing), nil
		}
		return l.autoPay(existing), nil
	}

	// 1. 大师公开详情（含服务标签）
	master, err := l.fetchMaster(masterCode)
	if err != nil {
		return nil, common.ErrMasterNotFound
	}

	// 2. 服务标签校验：大师提供该服务且已上架
	var tagPrice float64
	tagFound := false
	for _, t := range master.ServiceTags {
		if t.ServiceCode == req.ServiceCode && (t.Status == "" || t.Status == "enabled") {
			tagPrice = t.Price
			tagFound = true
			break
		}
	}
	if !tagFound {
		return nil, common.ErrConsultationUnavailable
	}

	// 3. 先付费咨询前置条件
	consulted, err := l.svcCtx.ConsultationModel.HasPaidConsultation(l.ctx, userIDStr, masterCode)
	if err != nil {
		l.Errorf("查询付费咨询记录失败: %v", err)
		return nil, common.ErrSystem
	}
	if !consulted {
		return nil, common.NewBizError(40415, "请先完成付费咨询后再预约服务")
	}

	// 4. 创建直约单（不占寺院时段库存）
	snapshot, _ := json.Marshal(map[string]interface{}{
		"masterCode": master.Code, "serviceCode": req.ServiceCode, "price": tagPrice,
	})
	created, err := l.svcCtx.BookingModel.InsertDirect(l.ctx, &model.Booking{
		RequestId:   req.RequestId,
		UserId:      userIDStr,
		TempleId:    master.TempleCode,
		TempleName:  master.TempleName,
		MasterId:    master.Code,
		MasterName:  master.DharmaName,
		ServiceId:   req.ServiceCode,
		ServiceName: req.ServiceCode,
		BookingDate: req.BookingDate,
		SlotCode:    "",
		TimeSlot:    "待协商",
		ServiceFee:  tagPrice,
		TotalFee:    tagPrice,
		PriceSnapshot: string(snapshot),
		Note:        req.Note,
	})
	if err != nil {
		if existing, findErr := l.svcCtx.BookingModel.FindByRequest(l.ctx, userIDStr, req.RequestId); findErr == nil {
			return responseFromBooking(existing), nil
		}
		l.Errorf("创建大师直约单失败: %v", err)
		return nil, common.ErrSystem
	}
	return l.autoPay(created), nil
}

func (l *DirectBookingLogic) autoPay(b *model.Booking) *types.CreateResp {
	return responseFromBooking(NewCreateLogic(l.ctx, l.svcCtx).autoPay(b))
}

// fetchMaster 经网关读取大师公开详情
func (l *DirectBookingLogic) fetchMaster(masterCode string) (*masterDetail, error) {
	base := strings.TrimRight(l.svcCtx.Config.GatewayBaseURL, "/")
	if base == "" {
		base = "http://localhost:8080"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("%s/api/v1/masters/%s", base, masterCode))
	if err != nil {
		l.Errorf("读取大师详情失败: %v", err)
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("master detail status %d", resp.StatusCode)
	}
	var env masterDetailEnvelope
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return nil, err
	}
	if env.Code != 0 || env.Data.Code == "" {
		return nil, fmt.Errorf("master not found")
	}
	return &env.Data, nil
}

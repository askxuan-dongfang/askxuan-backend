package logic

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
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
		if req.ManualPayment {
			return responseFromBooking(existing), nil
		}
		if err := requireValidConsultation(l.ctx, l.svcCtx, userIDStr, existing.MasterId); err != nil {
			return nil, err
		}
		return l.autoPay(existing), nil
	}

	// 1. 大师公开详情（含服务标签）
	master, err := l.fetchMaster(masterCode)
	if err != nil {
		return nil, common.ErrMasterNotFound
	}
	if master.ManageBy != "platform" {
		return nil, common.NewBizError(40312, "寺院师傅请通过寺院排期预约，费用包含寺院服务费与师傅执行费")
	}

	// 2. 服务标签校验：大师提供该服务且已上架
	tagPrice, tagFound := masterServicePrice(master, req.ServiceCode)
	if !tagFound {
		return nil, common.ErrConsultationUnavailable
	}

	// Both master booking paths require a currently valid paid consultation.
	if err := requireValidConsultation(l.ctx, l.svcCtx, userIDStr, masterCode); err != nil {
		return nil, err
	}

	// 4. 创建直约单（不占寺院时段库存）
	snapshot, _ := json.Marshal(map[string]interface{}{
		"masterCode": master.Code, "serviceCode": req.ServiceCode, "price": tagPrice,
	})
	created, err := l.svcCtx.BookingModel.InsertDirect(l.ctx, &model.Booking{
		RequestId:     req.RequestId,
		UserId:        userIDStr,
		TempleId:      master.TempleCode,
		TempleName:    master.TempleName,
		MasterId:      master.Code,
		MasterName:    master.DharmaName,
		ServiceId:     req.ServiceCode,
		ServiceName:   req.ServiceCode,
		BookingDate:   req.BookingDate,
		SlotCode:      "",
		TimeSlot:      "待协商",
		ServiceFee:    tagPrice,
		TotalFee:      tagPrice,
		PriceSnapshot: string(snapshot),
		Note:          req.Note,
	})
	if err != nil {
		if existing, findErr := l.svcCtx.BookingModel.FindByRequest(l.ctx, userIDStr, req.RequestId); findErr == nil {
			return responseFromBooking(existing), nil
		}
		l.Errorf("创建大师直约单失败: %v", err)
		return nil, common.ErrSystem
	}
	if req.ManualPayment {
		return responseFromBooking(created), nil
	}
	return l.autoPay(created), nil
}

func masterServicePrice(master *masterDetail, serviceCode string) (float64, bool) {
	for _, tag := range master.ServiceTags {
		if tag.ServiceCode == serviceCode && (tag.Status == "" || tag.Status == "enabled") && tag.Price >= 0 && !math.IsNaN(tag.Price) && !math.IsInf(tag.Price, 0) {
			return tag.Price, true
		}
	}
	return 0, false
}

func (l *DirectBookingLogic) autoPay(b *model.Booking) *types.CreateResp {
	return responseFromBooking(NewCreateLogic(l.ctx, l.svcCtx).autoPay(b))
}

// fetchMaster 经网关读取大师公开详情
func (l *DirectBookingLogic) fetchMaster(masterCode string) (*masterDetail, error) {
	return fetchMasterDetail(l.svcCtx, masterCode)
}

// fetchMasterDetail 经网关读取大师公开详情（含服务标签）
func fetchMasterDetail(svcCtx *svc.ServiceContext, masterCode string) (*masterDetail, error) {
	base := strings.TrimRight(svcCtx.Config.GatewayBaseURL, "/")
	if base == "" {
		base = "http://localhost:8080"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("%s/api/v1/masters/%s", base, masterCode))
	if err != nil {
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

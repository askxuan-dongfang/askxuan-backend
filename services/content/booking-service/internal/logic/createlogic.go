package logic

import (
	"context"

	"github.com/askxuan/booking-service/internal/mq"
	"github.com/askxuan/booking-service/internal/model"
	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/booking-service/internal/types"
	"github.com/askxuan/common"

	"github.com/zeromicro/go-zero/core/logx"
)

// CreateLogic 创建预约逻辑
type CreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateLogic {
	return &CreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Create 创建预约
// 1. 参数校验 2. 校验寺院/法师存在性 3. 落库（MySQL） 4. 记录初始状态日志 5. 发送 booking.notify 事件（不阻塞主流程）
func (l *CreateLogic) Create(req *types.CreateReq) (*types.CreateResp, error) {
	if req.UserId == "" || req.TempleId == "" || req.MasterId == "" || req.BookingDate == "" {
		return nil, common.ErrParam
	}

	// 校验寺院存在且状态正常
	temple, err := l.svcCtx.TempleReadonlyModel.FindByCode(l.ctx, req.TempleId)
	if err != nil {
		l.Errorf("查询寺院失败 templeId=%s: %v", req.TempleId, err)
		return nil, common.ErrTempleNotFound
	}
	if temple.Status != "正常" {
		return nil, common.NewBizError(40307, "寺院当前不可预约")
	}

	// 校验法师存在且已上架
	master, err := l.svcCtx.MasterReadonlyModel.FindByCode(l.ctx, req.MasterId)
	if err != nil {
		l.Errorf("查询法师失败 masterId=%s: %v", req.MasterId, err)
		return nil, common.ErrMasterNotFound
	}
	if master.ShelfStatus != "on_shelf" {
		return nil, common.NewBizError(40308, "法师当前不可预约")
	}

	created, err := l.svcCtx.BookingModel.Insert(l.ctx, &model.Booking{
		UserId:         req.UserId,
		TempleId:       req.TempleId,
		TempleName:     temple.Name,
		MasterId:       req.MasterId,
		MasterName:     master.DharmaName,
		ServiceId:      req.ServiceId,
		ServiceName:    req.ServiceName,
		BookingDate:    req.BookingDate,
		TimeSlot:       req.TimeSlot,
		MeritMoney:     req.MeritMoney,
		MeritMoneyTier: req.MeritMoneyTier,
		Note:           req.Note,
	})
	if err != nil {
		l.Errorf("创建预约失败: %v", err)
		return nil, common.ErrSystem
	}

	// 记录初始状态日志（pending 创建，失败不阻断主流程）
	if logErr := l.svcCtx.StatusLogModel.Insert(l.ctx, &model.BookingStatusLog{
		BookingId:    created.Id,
		FromStatus:   "",
		ToStatus:     model.StatusPending,
		OperatorId:   req.UserId,
		OperatorType: model.OperatorTypeUser,
		Remark:       "用户创建预约",
	}); logErr != nil {
		l.Errorf("记录初始状态日志失败: %v", logErr)
	}

	// 异步发送 booking.notify 事件，供 message-service 消费生成站内消息
	if l.svcCtx.MqProducer != nil {
		if err := l.svcCtx.MqProducer.Publish(l.ctx, mq.BookingNotify{
			BookingId: created.Id,
			UserId:    created.UserId,
			TempleId:  created.TempleId,
			Action:    "created",
		}); err != nil {
			l.Errorf("发送预约通知失败，不影响主流程: %v", err)
		}
	}

	return &types.CreateResp{
		Id:     created.Id,
		Status: created.Status,
	}, nil
}

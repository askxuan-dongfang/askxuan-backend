package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/diy-service/internal/model"
	"github.com/askxuan/diy-service/internal/mq"
	"github.com/askxuan/diy-service/internal/svc"
	"github.com/askxuan/diy-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// AdminDiyOrderListLogic 商城台DIY订单列表
type AdminDiyOrderListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminDiyOrderListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminDiyOrderListLogic {
	return &AdminDiyOrderListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminDiyOrderListLogic) List(req *types.AdminDiyOrderListReq) (*types.AdminDiyOrderListResp, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Size <= 0 {
		req.Size = 20
	}
	list, total, err := l.svcCtx.DiyOrderModel.FindListAdmin(l.ctx, req.Status, req.Page, req.Size)
	if err != nil {
		l.Errorf("查询DIY订单列表失败: %v", err)
		return nil, common.ErrSystem
	}
	resp := &types.AdminDiyOrderListResp{Total: total, Page: req.Page, Size: req.Size}
	for _, o := range list {
		resp.List = append(resp.List, toTypesDiyOrder(o, nil, model.BlessingTask{}))
	}
	return resp, nil
}

// AdminDiyOrderDetailLogic 商城台DIY订单详情
type AdminDiyOrderDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminDiyOrderDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminDiyOrderDetailLogic {
	return &AdminDiyOrderDetailLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminDiyOrderDetailLogic) Detail(req *types.AdminDiyOrderDetailReq) (*types.DiyOrder, error) {
	o, err := l.svcCtx.DiyOrderModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if err == sqlx.ErrNotFound {
			return nil, common.ErrDiyOrderNotFound
		}
		l.Errorf("查询DIY订单详情失败: %v", err)
		return nil, common.ErrSystem
	}
	return toTypesDiyOrderDetail(l.ctx, l.svcCtx, o), nil
}

// AdminDiyOrderReviewLogic 商城台审核设计
type AdminDiyOrderReviewLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminDiyOrderReviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminDiyOrderReviewLogic {
	return &AdminDiyOrderReviewLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminDiyOrderReviewLogic) Review(req *types.AdminDiyOrderReviewReq) (*types.DiyOrder, error) {
	o, err := l.svcCtx.DiyOrderModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if err == sqlx.ErrNotFound {
			return nil, common.ErrDiyOrderNotFound
		}
		return nil, common.ErrSystem
	}
	if o.Status != model.DiyStatusPendingReview {
		return nil, common.ErrStatusInvalid
	}

	var targetStatus string
	switch req.Action {
	case "approve":
		if o.PaymentStatus != "success" {
			return nil, common.NewBizError(common.ErrOrderStatusConflict.Code, "订单尚未支付，不能进入制作")
		}
		targetStatus = model.DiyStatusInMaking
	case "reject":
		updated, cancelErr := l.svcCtx.DiyOrderModel.CancelAndRestock(l.ctx, req.Id)
		if cancelErr != nil {
			if cancelErr == model.ErrDiyOrderStateConflict {
				return nil, common.ErrStatusInvalid
			}
			return nil, common.ErrSystem
		}
		return toTypesDiyOrderDetail(l.ctx, l.svcCtx, updated), nil
	default:
		return nil, common.ErrParam
	}

	if !model.CanDiyTransit(o.Status, targetStatus) {
		return nil, common.ErrStatusInvalid
	}

	updated, err := l.svcCtx.DiyOrderModel.UpdateStatusIfCurrent(l.ctx, req.Id, model.DiyStatusPendingReview, targetStatus)
	if err != nil {
		if err == model.ErrDiyOrderStateConflict {
			return nil, common.ErrStatusInvalid
		}
		return nil, common.ErrSystem
	}
	return toTypesDiyOrderDetail(l.ctx, l.svcCtx, updated), nil
}

// AdminDiyOrderMakeCompleteLogic 商城台制作完成
type AdminDiyOrderMakeCompleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminDiyOrderMakeCompleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminDiyOrderMakeCompleteLogic {
	return &AdminDiyOrderMakeCompleteLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminDiyOrderMakeCompleteLogic) MakeComplete(req *types.AdminDiyOrderMakeCompleteReq) (*types.DiyOrder, error) {
	o, err := l.svcCtx.DiyOrderModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if err == sqlx.ErrNotFound {
			return nil, common.ErrDiyOrderNotFound
		}
		return nil, common.ErrSystem
	}

	// 查 design 判断是否有加持
	var blessServiceCode string
	if design, derr := l.svcCtx.DiyDesignModel.FindOne(l.ctx, o.DesignId); derr == nil {
		blessServiceCode = design.BlessServiceCode
	}
	hasBless := blessServiceCode != ""

	var targetStatus string
	if hasBless {
		targetStatus = model.DiyStatusAwaitingBlessing
	} else {
		targetStatus = model.DiyStatusAwaitingShipment
	}
	if !model.CanDiyTransit(o.Status, targetStatus) {
		return nil, common.ErrStatusInvalid
	}

	updated, err := l.svcCtx.DiyOrderModel.UpdateStatus(l.ctx, req.Id, targetStatus)
	if err != nil {
		return nil, common.ErrSystem
	}

	// 如有加持，创建 blessing_task 并发 MQ 派单
	var blessTask model.BlessingTask
	if hasBless {
		var templeCode, masterCode string
		services, _ := l.svcCtx.ExtraServiceModel.FindList(l.ctx, 1, 100)
		for _, s := range services {
			if s.Code == blessServiceCode {
				templeCode = s.TempleCode
				masterCode = s.MasterCode
				break
			}
		}
		task, terr := l.svcCtx.BlessingTaskModel.Insert(l.ctx, &model.BlessingTask{
			DiyOrderNo: o.OrderNo,
			TempleCode: templeCode,
			MasterCode: masterCode,
			Status:     model.BlessingTaskStatusDispatched,
		})
		if terr != nil {
			l.Errorf("创建 blessing_task 失败(orderNo=%s): %v", o.OrderNo, terr)
		} else {
			blessTask = *task
			l.Infof("创建 blessing_task 成功(id=%d, taskNo=%s, orderNo=%s)", task.Id, task.TaskNo, o.OrderNo)
			_ = l.svcCtx.MqProducer.PublishBlessingDispatch(l.ctx, mq.BlessingDispatch{
				TaskNo:      task.TaskNo,
				DiyOrderId:  o.OrderNo,
				TempleCode:  templeCode,
				MasterCode:  masterCode,
				ServiceCode: blessServiceCode,
			})
		}
	}

	return toTypesDiyOrderDetailWithTask(l.ctx, l.svcCtx, updated, blessTask), nil
}

// AdminDiyOrderShipLogic 商城台发货
type AdminDiyOrderShipLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminDiyOrderShipLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminDiyOrderShipLogic {
	return &AdminDiyOrderShipLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminDiyOrderShipLogic) Ship(req *types.AdminDiyOrderShipReq) (*types.DiyOrder, error) {
	o, err := l.svcCtx.DiyOrderModel.FindOne(l.ctx, req.Id)
	if err != nil {
		if err == sqlx.ErrNotFound {
			return nil, common.ErrDiyOrderNotFound
		}
		return nil, common.ErrSystem
	}

	if !model.CanDiyTransit(o.Status, model.DiyStatusShipped) {
		return nil, common.ErrStatusInvalid
	}

	updated, err := l.svcCtx.DiyOrderModel.UpdateStatus(l.ctx, req.Id, model.DiyStatusShipped)
	if err != nil {
		return nil, common.ErrSystem
	}

	// 发 MQ order.events 通知 logistics-service 创建物流追踪记录
	_ = l.svcCtx.MqProducer.PublishOrderShipped(l.ctx, mq.OrderShippedNotify{
		OrderId: o.OrderNo,
		UserId:  o.UserId,
		Action:  "shipped",
	})

	return toTypesDiyOrderDetail(l.ctx, l.svcCtx, updated), nil
}

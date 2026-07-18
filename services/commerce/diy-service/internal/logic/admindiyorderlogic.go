package logic

import (
	"context"
	"encoding/json"

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

	// 下单时的不可变设计快照决定是否加持，避免作品后续编辑影响历史订单。
	var blessServiceCode string
	if o.DesignSnapshot != "" {
		var snapshot model.DiyDesign
		if err := json.Unmarshal([]byte(o.DesignSnapshot), &snapshot); err == nil {
			blessServiceCode = snapshot.BlessServiceCode
		}
	}
	// Legacy orders did not persist a design snapshot.
	if blessServiceCode == "" && o.DesignSnapshot == "" {
		if design, designErr := l.svcCtx.DiyDesignModel.FindOne(l.ctx, o.DesignId); designErr == nil {
			blessServiceCode = design.BlessServiceCode
		}
	}
	targetStatus := model.DiyStatusAwaitingShipment
	if blessServiceCode != "" {
		targetStatus = model.DiyStatusAwaitingBlessing
	}
	if !model.CanDiyTransit(o.Status, targetStatus) {
		return nil, common.ErrStatusInvalid
	}

	updated, task, err := l.svcCtx.DiyOrderModel.CompleteMaking(l.ctx, req.Id, blessServiceCode)
	if err != nil {
		if err == model.ErrDiyOrderStateConflict {
			return nil, common.ErrStatusInvalid
		}
		if err == model.ErrOrderBlessingUnavailable {
			return nil, common.NewBizError(40908, "加持服务已下架，请先调整订单")
		}
		l.Errorf("完成制作事务失败(orderNo=%s): %v", o.OrderNo, err)
		return nil, common.ErrSystem
	}

	var blessTask model.BlessingTask
	if task != nil {
		blessTask = *task
		l.Infof("创建 blessing_task 成功(id=%d, taskNo=%s, orderNo=%s)", task.Id, task.TaskNo, o.OrderNo)
		_ = l.svcCtx.MqProducer.PublishBlessingDispatch(l.ctx, mq.BlessingDispatch{
			TaskNo:      task.TaskNo,
			DiyOrderId:  o.OrderNo,
			TempleCode:  task.TempleCode,
			MasterCode:  task.MasterCode,
			ServiceCode: blessServiceCode,
		})
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

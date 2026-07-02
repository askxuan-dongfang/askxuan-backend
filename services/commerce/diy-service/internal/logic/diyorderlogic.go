package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/diy-service/internal/model"
	"github.com/askxuan/diy-service/internal/svc"
	"github.com/askxuan/diy-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// DiyOrderCreateLogic 创建DIY订单
type DiyOrderCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDiyOrderCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DiyOrderCreateLogic {
	return &DiyOrderCreateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *DiyOrderCreateLogic) Create(req *types.DiyOrderCreateReq) (*types.DiyOrderCreateResp, error) {
	if req.DesignId == 0 || len(req.Items) == 0 {
		return nil, common.ErrParam
	}

	// 校验设计存在
	_, err := l.svcCtx.DiyDesignModel.FindOne(l.ctx, req.DesignId)
	if err != nil {
		if err == sqlx.ErrNotFound {
			return nil, ErrDesignNotFound
		}
		return nil, common.ErrSystem
	}

	// 计算材料费
	var materialFee float64
	for _, it := range req.Items {
		materialFee += it.UnitPrice * float64(it.Quantity)
	}

	// 计算加持费（如有 blessServiceCode，查 extra_service 获取价格）
	var blessFee float64
	if req.BlessServiceCode != "" {
		services, serr := l.svcCtx.ExtraServiceModel.FindList(l.ctx, 1, 100)
		if serr == nil {
			for _, s := range services {
				if s.Code == req.BlessServiceCode {
					blessFee = s.Price
					break
				}
			}
		}
	}
	totalFee := materialFee + blessFee

	// 事务写 order + items
	var orderNo string
	var orderId int64
	err = l.svcCtx.DB.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		o, err := l.svcCtx.DiyOrderModel.Insert(l.ctx, &model.DiyOrder{
			UserId:      req.UserId,
			DesignId:    req.DesignId,
			MaterialFee: materialFee,
			BlessFee:    blessFee,
			TotalFee:    totalFee,
			Status:      model.DiyStatusPendingReview,
			AddressId:   req.AddressId,
		})
		if err != nil {
			return err
		}
		orderNo = o.OrderNo
		orderId = o.Id
		for _, it := range req.Items {
			_, err := l.svcCtx.DiyOrderItemModel.Insert(l.ctx, &model.DiyOrderItem{
				OrderId:      o.Id,
				MaterialId:   it.MaterialId,
				MaterialName: it.MaterialName,
				Spec:         it.Spec,
				UnitPrice:    it.UnitPrice,
				Quantity:     it.Quantity,
				Subtype:      it.Subtype,
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		l.Errorf("创建DIY订单失败: %v", err)
		return nil, common.ErrSystem
	}

	return &types.DiyOrderCreateResp{Id: orderId, OrderNo: orderNo}, nil
}

// DiyOrderListLogic 我的DIY订单列表
type DiyOrderListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDiyOrderListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DiyOrderListLogic {
	return &DiyOrderListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *DiyOrderListLogic) List(req *types.DiyOrderListReq) (*types.DiyOrderListResp, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Size <= 0 {
		req.Size = 20
	}
	list, total, err := l.svcCtx.DiyOrderModel.FindListByUser(l.ctx, req.UserId, req.Status, req.Page, req.Size)
	if err != nil {
		l.Errorf("查询DIY订单列表失败: %v", err)
		return nil, common.ErrSystem
	}
	resp := &types.DiyOrderListResp{Total: total, Page: req.Page, Size: req.Size}
	for _, o := range list {
		resp.List = append(resp.List, toTypesDiyOrder(o, nil, model.BlessingTask{}))
	}
	return resp, nil
}

// DiyOrderDetailLogic DIY订单详情
type DiyOrderDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDiyOrderDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DiyOrderDetailLogic {
	return &DiyOrderDetailLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *DiyOrderDetailLogic) Detail(req *types.DiyOrderDetailReq) (*types.DiyOrder, error) {
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

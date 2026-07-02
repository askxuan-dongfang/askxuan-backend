package logic

import (
	"context"
	"errors"

	"github.com/askxuan/common"
	"github.com/askxuan/user-service/internal/model"
	"github.com/askxuan/user-service/internal/svc"
	"github.com/askxuan/user-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ============ 收货地址管理 ============

// AddressListLogic 地址列表
type AddressListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAddressListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddressListLogic {
	return &AddressListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AddressListLogic) AddressList(userId int64) (*types.AddressListResp, error) {
	if userId == 0 {
		return nil, common.ErrUnauthorized
	}
	list, err := l.svcCtx.AddressModel.ListByUser(l.ctx, userId)
	if err != nil {
		return nil, common.ErrSystem
	}
	out := make([]types.UserAddress, 0, len(list))
	for _, a := range list {
		out = append(out, addrToType(a))
	}
	return &types.AddressListResp{List: out}, nil
}

// AddressCreateLogic 添加地址
type AddressCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAddressCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddressCreateLogic {
	return &AddressCreateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AddressCreateLogic) AddressCreate(req *types.AddressCreateReq, userId int64) (*types.AddressCreateResp, error) {
	if userId == 0 {
		return nil, common.ErrUnauthorized
	}
	if req.Name == "" || req.Phone == "" || req.Detail == "" {
		return nil, common.ErrParam
	}
	isDefault := 0
	if req.IsDefault {
		isDefault = 1
	}
	id, err := l.svcCtx.AddressModel.Insert(l.ctx, &model.UserAddress{
		UserId:    userId,
		Name:      req.Name,
		Phone:     req.Phone,
		Province:  req.Province,
		City:      req.City,
		District:  req.District,
		Detail:    req.Detail,
		IsDefault: isDefault,
	})
	if err != nil {
		return nil, common.ErrSystem
	}
	return &types.AddressCreateResp{Id: id}, nil
}

// AddressUpdateLogic 更新地址
type AddressUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAddressUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddressUpdateLogic {
	return &AddressUpdateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AddressUpdateLogic) AddressUpdate(req *types.AddressUpdateReq, userId int64) (*types.UserAddress, error) {
	if userId == 0 {
		return nil, common.ErrUnauthorized
	}
	// 查询地址是否存在且属于该用户
	a, err := l.svcCtx.AddressModel.FindByID(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.NewBizError(40401, "地址不存在")
		}
		return nil, common.ErrSystem
	}
	if a.UserId != userId {
		return nil, common.ErrForbidden
	}

	// 合并更新字段（空值保留原值）
	name := req.Name
	if name == "" {
		name = a.Name
	}
	phone := req.Phone
	if phone == "" {
		phone = a.Phone
	}
	province := req.Province
	if province == "" {
		province = a.Province
	}
	city := req.City
	if city == "" {
		city = a.City
	}
	district := req.District
	if district == "" {
		district = a.District
	}
	detail := req.Detail
	if detail == "" {
		detail = a.Detail
	}
	isDefault := a.IsDefault
	if req.IsDefault {
		isDefault = 1
	}

	if err := l.svcCtx.AddressModel.Update(l.ctx, &model.UserAddress{
		Id:        req.Id,
		UserId:    userId,
		Name:      name,
		Phone:     phone,
		Province:  province,
		City:      city,
		District:  district,
		Detail:    detail,
		IsDefault: isDefault,
	}); err != nil {
		return nil, common.ErrSystem
	}

	// 查询返回最新地址
	updated, err := l.svcCtx.AddressModel.FindByID(l.ctx, req.Id)
	if err != nil {
		return nil, common.ErrSystem
	}
	resp := addrToType(updated)
	return &resp, nil
}

// AddressDeleteLogic 删除地址
type AddressDeleteLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAddressDeleteLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddressDeleteLogic {
	return &AddressDeleteLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AddressDeleteLogic) AddressDelete(req *types.AddressDeleteReq, userId int64) error {
	if userId == 0 {
		return common.ErrUnauthorized
	}
	// 校验地址归属
	a, err := l.svcCtx.AddressModel.FindByID(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return common.NewBizError(40401, "地址不存在")
		}
		return common.ErrSystem
	}
	if a.UserId != userId {
		return common.ErrForbidden
	}
	if err := l.svcCtx.AddressModel.Delete(l.ctx, req.Id); err != nil {
		return common.ErrSystem
	}
	return nil
}

// addrToType 将 model.UserAddress 转换为 types.UserAddress
func addrToType(a *model.UserAddress) types.UserAddress {
	return types.UserAddress{
		Id:         a.Id,
		UserId:     a.UserId,
		Name:       a.Name,
		Phone:      a.Phone,
		Province:   a.Province,
		City:       a.City,
		District:   a.District,
		Detail:     a.Detail,
		IsDefault:  a.IsDefault == 1,
		CreateTime: a.CreateTime,
	}
}

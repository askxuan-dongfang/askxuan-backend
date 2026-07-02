package logic

import (
	"context"
	"errors"

	"github.com/askxuan/auth-service/internal/model"
	"github.com/askxuan/auth-service/internal/svc"
	"github.com/askxuan/auth-service/internal/types"
	"github.com/askxuan/common"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ============ 账号管理 ============

// AdminAccountListLogic 账号列表
type AdminAccountListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminAccountListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminAccountListLogic {
	return &AdminAccountListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminAccountListLogic) AdminAccountList(req *types.AdminAccountListReq) (*types.AdminAccountListResp, error) {
	page := req.Page
	size := req.Size
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}

	list, total, err := l.svcCtx.AdminAccountModel.FindList(l.ctx, req.Keyword, req.Status, page, size)
	if err != nil {
		l.Errorf("查询账号列表失败: %v", err)
		return nil, common.ErrSystem
	}

	// 批量查询角色名称
	roleMap := make(map[int64]string)
	for _, a := range list {
		if _, ok := roleMap[a.RoleId]; !ok {
			if r, err := l.svcCtx.RoleModel.FindByID(l.ctx, a.RoleId); err == nil {
				roleMap[a.RoleId] = r.Name
			}
		}
	}

	out := make([]types.AdminAccount, 0, len(list))
	for _, a := range list {
		out = append(out, types.AdminAccount{
			Id:            a.Id,
			Account:       a.Account,
			Name:          a.Name,
			RoleId:        a.RoleId,
			RoleName:      roleMap[a.RoleId],
			TempleId:      a.TempleId,
			MasterId:      a.MasterId,
			ShopId:        a.ShopId,
			Status:        a.Status,
			LastLoginTime: a.LastLoginTime,
			CreateTime:    a.CreateTime,
		})
	}

	return &types.AdminAccountListResp{
		Total: total,
		List:  out,
		Page:  page,
		Size:  size,
	}, nil
}

// AdminAccountCreateLogic 创建账号
type AdminAccountCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminAccountCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminAccountCreateLogic {
	return &AdminAccountCreateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminAccountCreateLogic) AdminAccountCreate(req *types.AdminAccountCreateReq) (*types.AdminAccountCreateResp, error) {
	// 账号去重
	exist, err := l.svcCtx.AdminAccountModel.FindByAccount(l.ctx, req.Account)
	if err != nil && !errors.Is(err, sqlx.ErrNotFound) {
		l.Errorf("查询账号失败 account=%s: %v", req.Account, err)
		return nil, common.ErrSystem
	}
	if exist != nil {
		return nil, common.ErrUserAlreadyExists
	}

	// MVP-1 明文存储密码
	id, err := l.svcCtx.AdminAccountModel.Insert(l.ctx, &model.AdminAccount{
		Account:  req.Account,
		Password: req.Password,
		Name:     req.Name,
		RoleId:   req.RoleId,
		TempleId: req.TempleId,
		MasterId: req.MasterId,
		ShopId:   req.ShopId,
	})
	if err != nil {
		l.Errorf("创建账号失败: %v", err)
		return nil, common.ErrSystem
	}

	return &types.AdminAccountCreateResp{Id: id}, nil
}

// AdminAccountUpdateLogic 更新账号
type AdminAccountUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminAccountUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminAccountUpdateLogic {
	return &AdminAccountUpdateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminAccountUpdateLogic) AdminAccountUpdate(req *types.AdminAccountUpdateReq) (*types.AdminAccount, error) {
	// 先查询存在
	exist, err := l.svcCtx.AdminAccountModel.FindByID(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrUserNotFound
		}
		l.Errorf("查询账号失败 id=%d: %v", req.Id, err)
		return nil, common.ErrSystem
	}

	// 合并更新字段（请求中非零值/非空字段覆盖原值）
	updated := &model.AdminAccount{
		Id:       exist.Id,
		Name:     exist.Name,
		RoleId:   exist.RoleId,
		TempleId: exist.TempleId,
		MasterId: exist.MasterId,
		ShopId:   exist.ShopId,
	}
	if req.Name != "" {
		updated.Name = req.Name
	}
	if req.RoleId != 0 {
		updated.RoleId = req.RoleId
	}
	if req.TempleId != "" {
		updated.TempleId = req.TempleId
	}
	if req.MasterId != "" {
		updated.MasterId = req.MasterId
	}
	if req.ShopId != 0 {
		updated.ShopId = req.ShopId
	}

	if err := l.svcCtx.AdminAccountModel.Update(l.ctx, updated); err != nil {
		l.Errorf("更新账号失败 id=%d: %v", req.Id, err)
		return nil, common.ErrSystem
	}

	// 查询角色名
	roleName := ""
	if r, err := l.svcCtx.RoleModel.FindByID(l.ctx, updated.RoleId); err == nil {
		roleName = r.Name
	}

	return &types.AdminAccount{
		Id:            updated.Id,
		Account:       exist.Account,
		Name:          updated.Name,
		RoleId:        updated.RoleId,
		RoleName:      roleName,
		TempleId:      updated.TempleId,
		MasterId:      updated.MasterId,
		ShopId:        updated.ShopId,
		Status:        exist.Status,
		LastLoginTime: exist.LastLoginTime,
		CreateTime:    exist.CreateTime,
	}, nil
}

// AdminAccountStatusLogic 启停账号
type AdminAccountStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminAccountStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminAccountStatusLogic {
	return &AdminAccountStatusLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminAccountStatusLogic) AdminAccountStatus(req *types.AdminAccountStatusReq) (*types.AdminAccountStatusResp, error) {
	// 校验状态值
	if req.Status != model.AccountStatusEnabled && req.Status != model.AccountStatusDisabled {
		return nil, common.ErrParam
	}

	// 校验账号存在
	if _, err := l.svcCtx.AdminAccountModel.FindByID(l.ctx, req.Id); err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrUserNotFound
		}
		l.Errorf("查询账号失败 id=%d: %v", req.Id, err)
		return nil, common.ErrSystem
	}

	if err := l.svcCtx.AdminAccountModel.UpdateStatus(l.ctx, req.Id, req.Status); err != nil {
		l.Errorf("更新账号状态失败 id=%d: %v", req.Id, err)
		return nil, common.ErrSystem
	}

	return &types.AdminAccountStatusResp{
		Id:     req.Id,
		Status: req.Status,
	}, nil
}

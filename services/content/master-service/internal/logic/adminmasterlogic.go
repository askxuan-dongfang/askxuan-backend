package logic

import (
	"context"
	"errors"

	"github.com/askxuan/common"
	"github.com/askxuan/master-service/internal/model"
	"github.com/askxuan/master-service/internal/svc"
	"github.com/askxuan/master-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ============ 寺院管理台 - 法师管理 Logic ============

// AdminMasterListLogic 本寺院法师列表
type AdminMasterListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminMasterListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminMasterListLogic {
	return &AdminMasterListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// AdminMasterList 寺院查询本寺院法师列表（按 templeId + status 筛选 + 分页）
func (l *AdminMasterListLogic) AdminMasterList(req *types.AdminMasterListReq) (*types.AdminMasterListResp, error) {
	if req.TempleId == "" {
		return nil, common.ErrParamMissing
	}
	page := req.Page
	size := req.Size
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	list, total, err := l.svcCtx.MasterModel.FindList(l.ctx, req.TempleId, req.Status, page, size)
	if err != nil {
		return nil, err
	}

	out := make([]types.Master, 0, len(list))
	for _, m := range list {
		out = append(out, toTypeMaster(m))
	}
	return &types.AdminMasterListResp{
		Total: total,
		List:  out,
		Page:  page,
		Size:  size,
	}, nil
}

// AdminMasterCreateLogic 创建法师
type AdminMasterCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminMasterCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminMasterCreateLogic {
	return &AdminMasterCreateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// AdminMasterCreate 寺院添加法师（创建法师记录 + 触发资质审核流程）
func (l *AdminMasterCreateLogic) AdminMasterCreate(req *types.AdminMasterCreateReq) (*types.AdminMasterCreateResp, error) {
	if req.DharmaName == "" || req.TempleId == "" || req.Position == "" || req.Sect == "" || req.Type == "" {
		return nil, common.ErrParamMissing
	}
	if _, err := l.svcCtx.MasterModel.FindTempleNameByCode(l.ctx, req.TempleId); err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrTempleNotFound
		}
		return nil, err
	}

	code, err := l.svcCtx.MasterModel.NextCode(l.ctx)
	if err != nil {
		return nil, err
	}

	master := &model.Master{
		Code:           code,
		DharmaName:     req.DharmaName,
		LayName:        req.LayName,
		TempleCode:     req.TempleId,
		Position:       req.Position,
		Sect:           req.Sect,
		Type:           req.Type,
		AuthStatus:     model.MasterAuthStatusPending, // 新建法师默认待审核
		ShelfStatus:    model.MasterShelfStatusOffShelf,
		PlatformStatus: model.MasterPlatformStatusNormal,
		Specialties:    joinSpecialties(req.Specialties),
		Avatar:         req.Avatar,
	}

	_, err = l.svcCtx.MasterModel.Insert(l.ctx, master)
	if err != nil {
		return nil, err
	}

	// 触发资质审核流程：插入一条 pending 审核记录
	_, _ = l.svcCtx.MasterAuditModel.InsertAuditLog(l.ctx, &model.MasterAudit{
		MasterCode: code,
		TempleCode: req.TempleId,
	})

	return &types.AdminMasterCreateResp{Id: code}, nil
}

// AdminMasterUpdateLogic 更新法师信息
type AdminMasterUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminMasterUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminMasterUpdateLogic {
	return &AdminMasterUpdateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// AdminMasterUpdate 更新法师信息（req.Id 为法师编码 code）
func (l *AdminMasterUpdateLogic) AdminMasterUpdate(req *types.AdminMasterUpdateReq) (*types.Master, error) {
	master, err := l.svcCtx.MasterModel.FindByCode(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrMasterNotFound
		}
		return nil, err
	}

	// 部分字段更新
	if req.DharmaName != "" {
		master.DharmaName = req.DharmaName
	}
	if req.LayName != "" {
		master.LayName = req.LayName
	}
	if req.Position != "" {
		master.Position = req.Position
	}
	if req.Specialties != nil {
		master.Specialties = joinSpecialties(req.Specialties)
	}
	if req.Avatar != "" {
		master.Avatar = req.Avatar
	}

	if err := l.svcCtx.MasterModel.Update(l.ctx, master); err != nil {
		return nil, err
	}
	resp := toTypeMaster(master)
	return &resp, nil
}

// AdminMasterStatusLogic 法师上下架
type AdminMasterStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminMasterStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminMasterStatusLogic {
	return &AdminMasterStatusLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// AdminMasterStatus 法师上下架（req.Id 为法师编码 code）
func (l *AdminMasterStatusLogic) AdminMasterStatus(req *types.AdminMasterStatusReq) (*types.AdminMasterStatusResp, error) {
	if req.Status != model.MasterShelfStatusOnShelf && req.Status != model.MasterShelfStatusOffShelf {
		return nil, common.ErrParamInvalid
	}
	master, err := l.svcCtx.MasterModel.FindByCode(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrMasterNotFound
		}
		return nil, err
	}
	if err := l.svcCtx.MasterModel.UpdateShelfStatus(l.ctx, master.Id, req.Status); err != nil {
		return nil, err
	}
	return &types.AdminMasterStatusResp{Id: req.Id, Status: req.Status}, nil
}

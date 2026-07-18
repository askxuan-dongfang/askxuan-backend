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

// ============ 法师工作台 - 个人资料 Logic ============

// WorkspaceProfileGetLogic 获取法师资料
type WorkspaceProfileGetLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWorkspaceProfileGetLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WorkspaceProfileGetLogic {
	return &WorkspaceProfileGetLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// WorkspaceProfileGet 法师获取自己的资料
func (l *WorkspaceProfileGetLogic) WorkspaceProfileGet(req *types.MasterProfileReq) (*types.MasterProfileResp, error) {
	masterCode, err := currentMasterCode(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}

	master, err := l.svcCtx.MasterModel.FindByCode(l.ctx, masterCode)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrMasterNotFound
		}
		l.Errorf("查询法师资料失败: %v", err)
		return nil, common.ErrSystem
	}

	ext, err := l.svcCtx.MasterProfileExtModel.Find(l.ctx, masterCode)
	if err != nil {
		l.Errorf("查询法师资料扩展失败: %v", err)
		return nil, common.ErrSystem
	}
	return toProfileResp(master, ext), nil
}

// WorkspaceProfileUpdateLogic 更新法师资料
type WorkspaceProfileUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewWorkspaceProfileUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *WorkspaceProfileUpdateLogic {
	return &WorkspaceProfileUpdateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// WorkspaceProfileUpdate 法师更新可编辑字段（bio, specialties, avatar, pricing）
func (l *WorkspaceProfileUpdateLogic) WorkspaceProfileUpdate(req *types.MasterProfileUpdateReq) (*types.MasterProfileResp, error) {
	masterCode, err := currentMasterCode(l.ctx, l.svcCtx)
	if err != nil {
		return nil, err
	}

	master, err := l.svcCtx.MasterModel.FindByCode(l.ctx, masterCode)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrMasterNotFound
		}
		l.Errorf("查询法师资料失败: %v", err)
		return nil, common.ErrSystem
	}

	// 仅更新可编辑字段：specialties / avatar（通过 MasterModel.Update 持久化到 DB）
	if req.Specialties != nil {
		master.Specialties = joinSpecialties(req.Specialties)
	}
	if req.Avatar != "" {
		master.Avatar = req.Avatar
	}
	if err := l.svcCtx.MasterModel.Update(l.ctx, master); err != nil {
		l.Errorf("更新法师资料失败: %v", err)
		return nil, common.ErrSystem
	}

	ext, err := l.svcCtx.MasterProfileExtModel.Find(l.ctx, masterCode)
	if err != nil {
		l.Errorf("查询法师资料扩展失败: %v", err)
		return nil, common.ErrSystem
	}
	if req.Bio != "" {
		ext.Bio = req.Bio
	}
	if req.Pricing != "" {
		ext.Pricing = req.Pricing
	}
	ext.MasterCode = masterCode
	if err := l.svcCtx.MasterProfileExtModel.Upsert(l.ctx, ext); err != nil {
		l.Errorf("保存法师资料扩展失败: %v", err)
		return nil, common.ErrSystem
	}

	return toProfileResp(master, ext), nil
}

// toProfileResp 将 model.Master + MasterProfileExt 组装为 types.MasterProfileResp
func toProfileResp(m *model.Master, ext model.MasterProfileExt) *types.MasterProfileResp {
	return &types.MasterProfileResp{
		Id:          m.Code,
		DharmaName:  m.DharmaName,
		LayName:     m.LayName,
		TempleId:    m.TempleCode,
		Position:    m.Position,
		Sect:        m.Sect,
		Type:        m.Type,
		AuthStatus:  m.AuthStatus,
		Specialties: splitSpecialties(m.Specialties),
		Avatar:      m.Avatar,
		Bio:         ext.Bio,
		Pricing:     ext.Pricing,
		Rating:      m.Rating,
	}
}

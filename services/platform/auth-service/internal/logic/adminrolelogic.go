package logic

import (
	"context"
	"errors"
	"github.com/askxuan/common/adminpolicy"
	"regexp"
	"strings"

	"github.com/askxuan/auth-service/internal/model"
	"github.com/askxuan/auth-service/internal/svc"
	"github.com/askxuan/auth-service/internal/types"
	"github.com/askxuan/common"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ============ 角色管理 ============

// AdminRoleListLogic 角色列表
type AdminRoleListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminRoleListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminRoleListLogic {
	return &AdminRoleListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminRoleListLogic) AdminRoleList() (*types.RoleListResp, error) {
	list, err := l.svcCtx.RoleModel.AllRoles(l.ctx)
	if err != nil {
		l.Errorf("查询角色列表失败: %v", err)
		return nil, common.ErrSystem
	}

	out := make([]types.Role, 0, len(list))
	for _, r := range list {
		out = append(out, types.Role{
			Id: r.Id, AuthzScope: effectiveRoleScope(r), AuthzVersion: r.AuthzVersion, Editable: !adminpolicy.Builtin(r.Code) && effectiveRoleScope(r) == "platform",
			Name:        r.Name,
			Code:        r.Code,
			Description: r.Description,
			CreateTime:  r.CreateTime,
		})
	}

	return &types.RoleListResp{List: out}, nil
}

// AdminRoleCreateLogic 创建角色
type AdminRoleCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminRoleCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminRoleCreateLogic {
	return &AdminRoleCreateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminRoleCreateLogic) AdminRoleCreate(req *types.RoleCreateReq) (*types.RoleCreateResp, error) {
	req.Code = strings.TrimSpace(req.Code)
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || len(req.Name) > 64 || !regexp.MustCompile(`^[a-z][a-z0-9_]{2,49}$`).MatchString(req.Code) || adminpolicy.Builtin(req.Code) || strings.HasPrefix(req.Code, "platform_") || len(req.Description) > 255 {
		return nil, common.ErrParam
	}
	id, err := l.svcCtx.RoleModel.Insert(l.ctx, &model.Role{
		Name:        req.Name,
		Code:        req.Code,
		Description: req.Description,
	})
	if err != nil {
		l.Errorf("创建角色失败: %v", err)
		return nil, common.ErrSystem
	}

	return &types.RoleCreateResp{Id: id}, nil
}

// AdminRoleUpdateLogic 更新角色
type AdminRoleUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminRoleUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminRoleUpdateLogic {
	return &AdminRoleUpdateLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminRoleUpdateLogic) AdminRoleUpdate(req *types.RoleUpdateReq) (*types.Role, error) {
	// 先查询存在
	exist, err := l.svcCtx.RoleModel.FindByID(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrParam
		}
		l.Errorf("查询角色失败 id=%d: %v", req.Id, err)
		return nil, common.ErrSystem
	}

	// 合并更新字段
	updated := &model.Role{
		Id:          exist.Id,
		Name:        exist.Name,
		Description: exist.Description,
	}
	if req.Name != "" {
		updated.Name = req.Name
	}
	if req.Description != "" {
		updated.Description = req.Description
	}

	if err := l.svcCtx.RoleModel.Update(l.ctx, updated); err != nil {
		l.Errorf("更新角色失败 id=%d: %v", req.Id, err)
		return nil, common.ErrSystem
	}

	return &types.Role{
		Id: updated.Id, AuthzScope: effectiveRoleScope(exist), AuthzVersion: exist.AuthzVersion, Editable: !adminpolicy.Builtin(exist.Code) && effectiveRoleScope(exist) == "platform",
		Name:        updated.Name,
		Code:        exist.Code,
		Description: updated.Description,
		CreateTime:  exist.CreateTime,
	}, nil
}

// ============ 权限管理 ============

// AdminPermissionListLogic 权限列表
type AdminPermissionListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminPermissionListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminPermissionListLogic {
	return &AdminPermissionListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminPermissionListLogic) AdminPermissionList() (*types.PermissionListResp, error) {
	rows, err := l.svcCtx.PermissionModel.AllPermissions(l.ctx)
	if err != nil {
		return nil, common.ErrSystem
	}
	ids := map[string]int64{}
	for _, row := range rows {
		ids[row.Code] = row.Id
	}
	out := make([]types.Permission, 0, len(adminpolicy.Catalog))
	for _, p := range adminpolicy.Catalog {
		// Locked directory items have no grantable DB row and use zero, never a fake ID.
		out = append(out, types.Permission{Id: ids[p.Code], Code: p.Code, Name: p.Name, Resource: p.Resource, Action: p.Action, Assignable: p.Assignable})
	}
	return &types.PermissionListResp{List: out}, nil
}

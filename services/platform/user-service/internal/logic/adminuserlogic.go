package logic

import (
	"context"
	"errors"
	"strings"

	"github.com/askxuan/common"
	"github.com/askxuan/user-service/internal/model"
	"github.com/askxuan/user-service/internal/svc"
	"github.com/askxuan/user-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ============ 平台用户管理 ============

// statusToInt 将字符串状态转为 int（normal→1, banned→0）
func statusToInt(s string) int {
	if s == "banned" {
		return model.UserStatusBanned
	}
	return model.UserStatusNormal
}

// statusToString 将 int 状态转为字符串（1→normal, 0→banned）
func statusToString(s int) string {
	if s == model.UserStatusBanned {
		return "banned"
	}
	return "normal"
}

// maskMobile 手机号脱敏（13800138000 → 138****8000）
func maskMobile(mobile string) string {
	if len(mobile) < 7 {
		return mobile
	}
	return mobile[:3] + "****" + mobile[len(mobile)-4:]
}

// AdminUserListLogic 平台用户列表
type AdminUserListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminUserListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminUserListLogic {
	return &AdminUserListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminUserListLogic) AdminUserList(req *types.AdminUserListReq) (*types.AdminUserListResp, error) {
	page := req.Page
	if page <= 0 {
		page = 1
	}
	size := req.Size
	if size <= 0 {
		size = 20
	}

	// 构造过滤条件
	status := -1 // -1=全部
	if req.Status == "normal" {
		status = model.UserStatusNormal
	} else if req.Status == "banned" {
		status = model.UserStatusBanned
	}

	list, total, err := l.svcCtx.UserModel.FindList(l.ctx, model.UserFilter{
		Keyword: strings.TrimSpace(req.Keyword),
		Status:  status,
	}, page, size)
	if err != nil {
		return nil, common.ErrSystem
	}

	items := make([]types.AdminUserItem, 0, len(list))
	for _, u := range list {
		items = append(items, types.AdminUserItem{
			UserId:     u.Id,
			Nickname:   u.Nickname,
			Mobile:     maskMobile(u.Mobile),
			Avatar:     u.Avatar,
			Region:     u.Region,
			Status:     statusToString(u.Status),
			CreateTime: u.CreateTime,
		})
	}
	return &types.AdminUserListResp{
		Total: total,
		List:  items,
		Page:  page,
		Size:  size,
	}, nil
}

// AdminUserDetailLogic 用户详情（含画像）
type AdminUserDetailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminUserDetailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminUserDetailLogic {
	return &AdminUserDetailLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminUserDetailLogic) AdminUserDetail(req *types.AdminUserDetailReq) (*types.AdminUserDetailResp, error) {
	u, err := l.svcCtx.UserModel.FindByID(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrUserNotFound
		}
		return nil, common.ErrSystem
	}

	// 查询用户画像（可选，不存在则返回默认值）
	var tags []string
	var totalOrders int
	var totalSpent float64
	var lastActive string
	profile, err := l.svcCtx.ProfileModel.FindByID(l.ctx, req.Id)
	if err == nil && profile != nil {
		tags = model.SplitTags(profile.PreferenceTags)
		totalOrders = profile.TotalOrders
		totalSpent = profile.TotalSpent
		lastActive = profile.LastActiveTime
	}

	return &types.AdminUserDetailResp{
		User: types.UserProfile{
			UserId:   u.Id,
			Nickname: u.Nickname,
			Avatar:   u.Avatar,
			Mobile:   maskMobile(u.Mobile),
			Gender:   u.Gender,
			Birthday: u.Birthday,
			Region:   u.Region,
			Bio:      u.Bio,
		},
		PreferenceTags: tags,
		TotalOrders:    totalOrders,
		TotalSpent:     totalSpent,
		LastActiveTime: lastActive,
		Status:         statusToString(u.Status),
	}, nil
}

// AdminUserStatusLogic 封禁/解封
type AdminUserStatusLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminUserStatusLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminUserStatusLogic {
	return &AdminUserStatusLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

func (l *AdminUserStatusLogic) AdminUserStatus(req *types.AdminUserStatusReq) (*types.AdminUserStatusResp, error) {
	// 校验用户存在
	exist, err := l.svcCtx.UserModel.FindByID(l.ctx, req.Id)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrUserNotFound
		}
		return nil, common.ErrSystem
	}
	_ = exist

	// 更新状态
	if err := l.svcCtx.UserModel.UpdateStatus(l.ctx, req.Id, statusToInt(req.Status)); err != nil {
		return nil, common.ErrSystem
	}
	return &types.AdminUserStatusResp{
		Id:     req.Id,
		Status: req.Status,
	}, nil
}

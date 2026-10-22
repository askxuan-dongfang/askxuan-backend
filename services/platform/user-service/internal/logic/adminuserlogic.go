package logic

import (
	"context"
	"errors"
	"strings"
	"time"

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

	if size > 100 {
		size = 100
	}
	if req.Status != "" && req.Status != "normal" && req.Status != "banned" {
		return nil, common.ErrParam
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

	ids := make([]int64, 0, len(list))
	for _, u := range list {
		ids = append(ids, u.Id)
	}
	stats, err := model.ReadUserConsumption(l.ctx, l.svcCtx.DB, ids)
	if err != nil {
		l.Errorf("读取用户消费统计失败: %v", err)
		return nil, common.NewBizError(50003, "消费统计暂不可用，请稍后重试")
	}
	updatedAt := time.Now().UTC().Format(time.RFC3339)
	items := make([]types.AdminUserItem, 0, len(list))
	for _, u := range list {
		items = append(items, types.AdminUserItem{
			UserId:         u.Id,
			Nickname:       u.Nickname,
			Mobile:         maskMobile(u.Mobile),
			Avatar:         u.Avatar,
			Region:         u.Region,
			Status:         statusToString(u.Status),
			CreateTime:     u.CreateTime,
			TotalOrders:    stats[u.Id].TotalOrders,
			TotalSpent:     float64(stats[u.Id].SpentCents) / 100,
			StatsScope:     model.UserConsumptionScope,
			StatsUpdatedAt: updatedAt,
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
	tags := []string{}
	var lastActive string
	profile, err := l.svcCtx.ProfileModel.FindByID(l.ctx, req.Id)
	if err == nil && profile != nil {
		tags = model.SplitTags(profile.PreferenceTags)
		lastActive = profile.LastActiveTime
	}

	if err != nil && !errors.Is(err, sqlx.ErrNotFound) {
		return nil, common.ErrSystem
	}
	stats, err := model.ReadUserConsumption(l.ctx, l.svcCtx.DB, []int64{req.Id})
	if err != nil {
		l.Errorf("读取用户消费统计失败: %v", err)
		return nil, common.NewBizError(50003, "消费统计暂不可用，请稍后重试")
	}
	credential, err := model.ReadUserCredential(l.ctx, l.svcCtx.DB, req.Id)
	if err != nil {
		return nil, common.ErrSystem
	}
	if credential.CredentialState == "legacy_password" && u.Password == "" {
		credential.CredentialState = "unavailable"
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
		TotalOrders:    stats[req.Id].TotalOrders,
		TotalSpent:     float64(stats[req.Id].SpentCents) / 100,
		StatsScope:     model.UserConsumptionScope,
		StatsUpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Identity:       types.UserCredential{Email: credential.Email, EmailVerified: credential.EmailVerified, VerifiedAt: credential.VerifiedAt, CredentialState: credential.CredentialState, CanResetPassword: credential.CanResetPassword},
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
	if req.Id <= 0 || (req.Status != "normal" && req.Status != "banned") {
		return nil, common.ErrParam
	}
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

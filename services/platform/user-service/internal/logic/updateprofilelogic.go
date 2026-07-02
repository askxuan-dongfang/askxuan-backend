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

// UpdateProfileLogic 更新用户资料逻辑
type UpdateProfileLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateProfileLogic {
	return &UpdateProfileLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// UpdateProfile 更新用户资料，仅更新非空字段
func (l *UpdateProfileLogic) UpdateProfile(req *types.UpdateProfileReq) (*types.UserProfile, error) {
	if req.UserId == 0 {
		return nil, common.ErrUnauthorized
	}

	// 先查询用户是否存在
	exist, err := l.svcCtx.UserModel.FindByID(l.ctx, req.UserId)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrUserNotFound
		}
		return nil, common.ErrSystem
	}

	// 合并更新字段（空值保留原值）
	data := &model.User{
		Id:       req.UserId,
		Nickname: req.Nickname,
		Avatar:   req.Avatar,
		Gender:   req.Gender,
		Birthday: req.Birthday,
		Region:   req.Region,
		Bio:      req.Bio,
	}
	if data.Nickname == "" {
		data.Nickname = exist.Nickname
	}
	if data.Avatar == "" {
		data.Avatar = exist.Avatar
	}
	if data.Gender == "" {
		data.Gender = exist.Gender
	}
	if data.Birthday == "" {
		data.Birthday = exist.Birthday
	}
	if data.Region == "" {
		data.Region = exist.Region
	}
	if data.Bio == "" {
		data.Bio = exist.Bio
	}

	if err := l.svcCtx.UserModel.Update(l.ctx, data); err != nil {
		return nil, common.ErrSystem
	}

	// 查询返回最新资料
	updated, err := l.svcCtx.UserModel.FindByID(l.ctx, req.UserId)
	if err != nil {
		return nil, common.ErrSystem
	}
	resp := userToProfile(updated)
	return &resp, nil
}

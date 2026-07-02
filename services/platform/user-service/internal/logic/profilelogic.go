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

// ProfileLogic 查询用户资料逻辑
type ProfileLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewProfileLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ProfileLogic {
	return &ProfileLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Profile 查询用户资料，userId 由网关鉴权后透传
func (l *ProfileLogic) Profile(req *types.ProfileReq) (*types.UserProfile, error) {
	if req.UserId == 0 {
		return nil, common.ErrUnauthorized
	}
	u, err := l.svcCtx.UserModel.FindByID(l.ctx, req.UserId)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrUserNotFound
		}
		return nil, common.ErrSystem
	}
	resp := userToProfile(u)
	return &resp, nil
}

// userToProfile 将 model.User 转换为 types.UserProfile
func userToProfile(u *model.User) types.UserProfile {
	return types.UserProfile{
		UserId:   u.Id,
		Nickname: u.Nickname,
		Avatar:   u.Avatar,
		Mobile:   u.Mobile,
		Gender:   u.Gender,
		Birthday: u.Birthday,
		Region:   u.Region,
		Bio:      u.Bio,
	}
}

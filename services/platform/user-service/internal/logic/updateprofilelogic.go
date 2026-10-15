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

// UpdateProfile preserves omitted fields and applies explicit empty values.
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

	data, err := mergeProfileUpdate(exist, req)
	if err != nil {
		return nil, err
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

// mergeProfileUpdate distinguishes an omitted field from an explicit clear.
func mergeProfileUpdate(existing *model.User, req *types.UpdateProfileReq) (*model.User, error) {
	data := *existing
	if req.Nickname != nil {
		data.Nickname = strings.TrimSpace(*req.Nickname)
		if data.Nickname == "" {
			return nil, common.ErrParam
		}
	}
	if req.Avatar != nil {
		data.Avatar = *req.Avatar
	}
	if req.Gender != nil {
		data.Gender = *req.Gender
	}
	if req.Birthday != nil {
		data.Birthday = *req.Birthday
	}
	if req.Region != nil {
		data.Region = *req.Region
	}
	if req.Bio != nil {
		data.Bio = *req.Bio
	}
	return &data, nil
}

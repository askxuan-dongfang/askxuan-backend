package logic

import (
	"context"
	"errors"
	"strconv"

	"github.com/askxuan/common"
	"github.com/askxuan/user-service/internal/model"
	"github.com/askxuan/user-service/internal/svc"
	"github.com/askxuan/user-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// RegisterLogic 用户注册逻辑
type RegisterLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRegisterLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterLogic {
	return &RegisterLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// Register 用户注册
// MVP-1 阶段验证码固定 1234
func (l *RegisterLogic) Register(req *types.RegisterReq) (*types.RegisterResp, error) {
	if req.Mobile == "" {
		return nil, common.ErrParam
	}
	if req.Code != "1234" {
		return nil, common.NewBizError(40106, "验证码错误")
	}

	// 检查手机号是否已注册
	exist, err := l.svcCtx.UserModel.FindByMobile(l.ctx, req.Mobile)
	if err != nil && !errors.Is(err, sqlx.ErrNotFound) {
		return nil, common.ErrSystem
	}
	if exist != nil {
		return nil, common.ErrUserAlreadyExists
	}

	// 构造默认昵称
	nickname := req.Nickname
	if nickname == "" {
		nickname = "用户" + req.Mobile[len(req.Mobile)-4:]
	}

	// 插入用户（MVP-1 密码明文 123456，预留 bcrypt 升级路径）
	uid, err := l.svcCtx.UserModel.Insert(l.ctx, &model.User{
		Mobile:   req.Mobile,
		Password: "123456",
		Nickname: nickname,
		Avatar:   "/assets/master-avatar-zhihai.jpg",
		Gender:   "unknown",
	})
	if err != nil {
		return nil, common.ErrSystem
	}

	// best-effort 同步用户到 OpenIM（失败不影响注册主流程）
	if l.svcCtx.IMClient != nil {
		userIDStr := "u_" + strconv.FormatInt(uid, 10)
		if syncErr := l.svcCtx.IMClient.RegisterUser(l.ctx, userIDStr, nickname, "/assets/master-avatar-zhihai.jpg"); syncErr != nil {
			l.Errorf("同步用户到 OpenIM 失败 uid=%d: %v", uid, syncErr)
		}
	}

	return &types.RegisterResp{UserId: uid}, nil
}

package logic

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/askxuan/common"
	"github.com/askxuan/user-service/internal/model"
	"github.com/askxuan/user-service/internal/svc"
	"github.com/askxuan/user-service/internal/types"
	"github.com/go-sql-driver/mysql"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var mainlandMobilePattern = regexp.MustCompile(`^1[3-9][0-9]{9}$`)

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

// Register 用户注册。演示环境不接真实短信，Code 仅作为旧客户端兼容字段。
func (l *RegisterLogic) Register(req *types.RegisterReq) (*types.RegisterResp, error) {
	mobile := strings.TrimSpace(req.Mobile)
	if !mainlandMobilePattern.MatchString(mobile) {
		return nil, common.ErrParamInvalid
	}

	// 检查手机号是否已注册
	exist, err := l.svcCtx.UserModel.FindByMobile(l.ctx, mobile)
	if err != nil && !errors.Is(err, sqlx.ErrNotFound) {
		return nil, common.ErrSystem
	}
	if exist != nil {
		return nil, common.ErrUserAlreadyExists
	}

	// 构造默认昵称
	nickname := strings.TrimSpace(req.Nickname)
	if nickname == "" {
		nickname = "善信" + mobile[len(mobile)-4:]
	}
	if len([]rune(nickname)) > 32 {
		return nil, common.ErrParamInvalid
	}

	// 插入用户（MVP-1 密码明文 123456，预留 bcrypt 升级路径）
	uid, err := l.svcCtx.UserModel.Insert(l.ctx, &model.User{
		Mobile:   mobile,
		Password: "123456",
		Nickname: nickname,
		Avatar:   "",
		Gender:   "unknown",
	})
	if err != nil {
		// 唯一索引兜住并发注册；预查询只用于提供更快的正常错误路径。
		var mysqlErr *mysql.MySQLError
		if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
			return nil, common.ErrUserAlreadyExists
		}
		l.Errorf("创建用户及画像失败 mobile=%s: %v", mobile, err)
		return nil, common.ErrSystem
	}

	// 注册主数据成功后同步 OpenIM。登录时还会再次幂等同步，便于依赖恢复后自愈。
	imReady := false
	if l.svcCtx.IMClient != nil {
		userIDStr := "u_" + strconv.FormatInt(uid, 10)
		if syncErr := l.svcCtx.IMClient.RegisterUser(l.ctx, userIDStr, nickname, ""); syncErr != nil {
			l.Errorf("同步用户到 OpenIM 失败 uid=%d: %v", uid, syncErr)
		} else {
			imReady = true
		}
	}

	return &types.RegisterResp{UserId: uid, Mobile: mobile, Nickname: nickname, IMReady: imReady}, nil
}

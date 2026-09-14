package svc

import (
	"context"

	"github.com/askxuan/auth-service/internal/config"
	"github.com/askxuan/auth-service/internal/model"
	"github.com/askxuan/common/identity"
	"github.com/askxuan/common/im"

	"github.com/zeromicro/go-zero/core/stores/redis"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// TokenBlacklist keeps session revocation checks replaceable by isolated test fixtures.
type TokenBlacklist interface {
	GetCtx(context.Context, string) (string, error)
	Setex(string, string, int) error
}

// ServiceContext auth 服务依赖容器
type ServiceContext struct {
	Accounts          *identity.Accounts
	SessionRedis      *redis.Redis
	Config            config.Config
	DB                sqlx.SqlConn
	Redis             TokenBlacklist
	UserReadonlyModel model.UserReadonlyModel
	AdminAccountModel model.AdminAccountModel
	RoleModel         model.RoleModel
	PermissionModel   model.PermissionModel
	IMClient          *im.Client
}

// NewServiceContext 初始化依赖
func NewServiceContext(c config.Config) *ServiceContext {
	db := sqlx.NewMysql(c.MySQL.DataSource)
	sc := &ServiceContext{
		Config:            c,
		DB:                db,
		Redis:             redis.MustNewRedis(c.Redis),
		UserReadonlyModel: model.NewUserReadonlyModel(db),
		AdminAccountModel: model.NewAdminAccountModel(db),
		RoleModel:         model.NewRoleModel(db),
		PermissionModel:   model.NewPermissionModel(db),
	}
	sc.SessionRedis = redis.MustNewRedis(c.Redis)
	sc.Accounts = &identity.Accounts{DB: db, Challenges: &identity.Challenges{Redis: sc.SessionRedis, Secret: c.Auth.AccessSecret}, Mailer: identity.SMTPFromEnv()}
	// OpenIM 可选：APIURL 为空时跳过（避免本地无 OpenIM 时登录卡住）
	if c.IM.APIURL != "" {
		sc.IMClient = im.NewClient(c.IM.APIURL, c.IM.AdminUserID, c.IM.Secret)
	}
	return sc
}

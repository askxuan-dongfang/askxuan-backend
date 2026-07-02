package svc

import (
	"github.com/askxuan/common/im"
	"github.com/askxuan/user-service/internal/config"
	"github.com/askxuan/user-service/internal/model"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ServiceContext user 服务依赖容器
type ServiceContext struct {
	Config       config.Config
	DB           sqlx.SqlConn
	UserModel    model.UserModel
	ProfileModel model.UserProfileModel
	AddressModel model.AddressModel
	IMClient     *im.Client
}

func NewServiceContext(c config.Config) *ServiceContext {
	db := sqlx.NewMysql(c.MySQL.DataSource)
	return &ServiceContext{
		Config:       c,
		DB:           db,
		UserModel:    model.NewUserModel(db),
		ProfileModel: model.NewUserProfileModel(db),
		AddressModel: model.NewAddressModel(db),
		IMClient:     im.NewClient(c.IM.APIURL, c.IM.AdminUserID, c.IM.Secret),
	}
}

package svc

import (
	"github.com/askxuan/community-service/internal/config"
	"github.com/askxuan/community-service/internal/model"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	DB    sqlx.SqlConn
	Model model.CommunityModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	db := sqlx.NewMysql(c.MySQL.DataSource)
	return &ServiceContext{DB: db, Model: model.NewCommunityModel(db)}
}

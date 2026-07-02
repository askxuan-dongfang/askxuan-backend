package svc

import (
	"github.com/askxuan/product-service/internal/config"
	"github.com/askxuan/product-service/internal/model"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ServiceContext product 服务依赖容器
type ServiceContext struct {
	Config               config.Config
	DB                   sqlx.SqlConn
	ProductModel         model.ProductModel
	ProductSkuModel      model.ProductSkuModel
	ProductCategoryModel model.ProductCategoryModel
	ProductImageModel    model.ProductImageModel
}

func NewServiceContext(c config.Config) *ServiceContext {
	db := sqlx.NewMysql(c.DataSource)
	return &ServiceContext{
		Config:               c,
		DB:                   db,
		ProductModel:         model.NewProductModel(db),
		ProductSkuModel:      model.NewProductSkuModel(db),
		ProductCategoryModel: model.NewProductCategoryModel(db),
		ProductImageModel:    model.NewProductImageModel(db),
	}
}

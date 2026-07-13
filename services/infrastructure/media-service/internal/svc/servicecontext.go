package svc

import (
	"os"
	"strconv"

	"github.com/askxuan/media-service/internal/config"
	"github.com/askxuan/media-service/internal/model"
	"github.com/askxuan/media-service/internal/provider"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ServiceContext struct {
	Config        config.Config
	DB            sqlx.SqlConn
	MediaModel    model.MediaModel
	LiveRoomModel model.LiveRoomModel
	MediaProvider provider.MediaProvider
	LiveProvider  provider.LiveProvider
	LiveEnabled   bool
	CallbackToken string
}

func NewServiceContext(c config.Config) *ServiceContext {
	if value := os.Getenv("LIVE_ENABLED"); value != "" {
		if enabled, err := strconv.ParseBool(value); err == nil {
			c.Live.Enabled = enabled
		}
	}
	if value := os.Getenv("LIVE_PROVIDER"); value != "" {
		c.Live.Provider = value
	}
	if value := os.Getenv("MEDIA_CALLBACK_TOKEN"); value != "" {
		c.Media.CallbackToken = value
	}
	mediaProvider, err := provider.NewMinIOProvider(c.MinIO)
	if err != nil {
		panic(err)
	}
	db := sqlx.NewMysql(c.MySQL.DataSource)
	return &ServiceContext{
		Config: c, DB: db,
		MediaModel: model.NewMediaModel(db), LiveRoomModel: model.NewLiveRoomModel(db),
		MediaProvider: mediaProvider,
		LiveProvider:  provider.DisabledLiveProvider{ProviderName: c.Live.Provider},
		LiveEnabled:   c.Live.Enabled, CallbackToken: c.Media.CallbackToken,
	}
}

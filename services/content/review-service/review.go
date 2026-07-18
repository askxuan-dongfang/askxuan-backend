package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/askxuan/review-service/internal/config"
	"github.com/askxuan/review-service/internal/handler"
	"github.com/askxuan/review-service/internal/model"
	"github.com/askxuan/review-service/internal/mq"
	"github.com/askxuan/review-service/internal/svc"

	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

var configFile = flag.String("f", "etc/review.yaml", "the config file")

func main() {
	flag.Parse()

	var c config.Config
	conf.MustLoad(*configFile, &c)

	server := rest.MustNewServer(c.RestConf)
	defer server.Stop()

	svcCtx := svc.NewServiceContext(c)
	consumerCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if svcCtx.MqConsumer != nil {
		svcCtx.MqConsumer.Start(consumerCtx, func(ctx context.Context, event mq.BookingReviewed) error {
			return model.UpsertBookingReview(ctx, event.BookingId, event.UserId, event.MasterId, event.Rating, event.ReviewContent, event.ReviewImages)
		})
	}
	handler.RegisterHandlers(server, svcCtx)

	fmt.Printf("启动 review-service，监听 %s:%d\n", c.Host, c.Port)
	server.Start()
}

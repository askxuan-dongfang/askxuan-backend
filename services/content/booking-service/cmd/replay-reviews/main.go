package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/askxuan/booking-service/internal/config"
	"github.com/askxuan/booking-service/internal/mq"
	"github.com/askxuan/common/mqoutbox"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func main() {
	file := flag.String("f", "etc/booking.yaml", "config")
	flag.Parse()
	var c config.Config
	conf.MustLoad(*file, &c)
	db := sqlx.NewMysql(c.MySQL.DataSource)
	ctx := context.Background()
	var rows []struct {
		BookingID   string `db:"booking_id"`
		UserID      string `db:"user_id"`
		TempleCode  string `db:"temple_code"`
		TempleName  string `db:"temple_name"`
		MasterCode  string `db:"master_code"`
		MasterName  string `db:"master_name"`
		ServiceName string `db:"service_name"`
		Rating      int    `db:"rating"`
		Content     string `db:"content"`
		Images      string `db:"images"`
		Reply       string `db:"master_reply"`
		Created     string `db:"created"`
	}
	err := db.QueryRowsCtx(ctx, &rows, `SELECT r.booking_id,r.user_id,b.temple_code,b.temple_name,b.master_code,b.master_name,b.service_name,r.rating,r.content,COALESCE(r.images,'[]') images,r.master_reply,DATE_FORMAT(r.create_time,'%Y-%m-%d %H:%i:%s') created FROM booking_review r JOIN booking b ON b.booking_no=r.booking_id`)
	if err != nil {
		panic(err)
	}
	for _, r := range rows {
		body, _ := json.Marshal(mq.BookingNotify{BookingId: r.BookingID, UserId: r.UserID, TempleId: r.TempleCode, TempleName: r.TempleName, MasterId: r.MasterCode, MasterName: r.MasterName, ServiceName: r.ServiceName, Rating: r.Rating, ReviewContent: r.Content, ReviewImages: r.Images, MasterReply: r.Reply, Time: r.Created, Action: "review_synced"})
		if err := mqoutbox.Enqueue(ctx, db, "review-repair-20260927:"+r.BookingID, "booking", r.BookingID, "booking.review_synced", mq.ExchangeBookingEvents, "", string(body)); err != nil {
			panic(err)
		}
	}
	fmt.Printf("Queued %d existing reviews for idempotent projection repair\n", len(rows))
}

package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/askxuan/review-service/internal/config"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func main() {
	file := flag.String("f", "etc/review.yaml", "config")
	flag.Parse()
	var c config.Config
	conf.MustLoad(*file, &c)
	db := sqlx.NewMysql(c.MySQL.DataSource)
	sqlx.DisableLog()
	_, err := db.ExecCtx(context.Background(), `-- Run with the existing review-service database account before deploying.
CREATE TABLE IF NOT EXISTS review_booking_context (
 booking_id VARCHAR(64) NOT NULL PRIMARY KEY,
 temple_code VARCHAR(64) NOT NULL DEFAULT '',
 temple_name VARCHAR(255) NOT NULL DEFAULT '',
 master_name VARCHAR(255) NOT NULL DEFAULT '',
 service_name VARCHAR(255) NOT NULL DEFAULT '',
 master_reply TEXT NOT NULL,
 INDEX idx_review_temple (temple_code)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
`)
	if err != nil {
		panic(err)
	}
	fmt.Println("review_booking_context ready")
}

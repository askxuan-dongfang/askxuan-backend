package model

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var db sqlx.SqlConn

func Configure(conn sqlx.SqlConn) { db = conn }

func paging(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	return (page - 1) * size, size
}

package model

import "github.com/zeromicro/go-zero/core/stores/sqlx"

var db sqlx.SqlConn

func Configure(conn sqlx.SqlConn) { db = conn }

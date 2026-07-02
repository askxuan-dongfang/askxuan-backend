package model

import (
	"context"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// pushLogTable 推送日志表（位于 askxuan_message 库）
const pushLogTable = "push_log"

// PushLog 推送日志实体
type PushLog struct {
	Id         int64  `db:"id" json:"id"`
	UserId     string `db:"user_id" json:"userId"`
	PushType   string `db:"push_type" json:"pushType"`
	Title      string `db:"title" json:"title"`
	Content    string `db:"content" json:"content"`
	Status     string `db:"status" json:"status"`
	BizType    string `db:"biz_type" json:"bizType"`
	BizId      string `db:"biz_id" json:"bizId"`
	CreateTime string `db:"create_time" json:"createTime"`
}

// PushLogModel 推送日志模型接口
type PushLogModel interface {
	Insert(ctx context.Context, p *PushLog) (int64, error)
}

type defaultPushLogModel struct {
	conn sqlx.SqlConn
}

// NewPushLogModel 构造推送日志模型
func NewPushLogModel(conn sqlx.SqlConn) PushLogModel {
	return &defaultPushLogModel{conn: conn}
}

// Insert 新建推送日志，返回自增 ID
func (m *defaultPushLogModel) Insert(ctx context.Context, data *PushLog) (int64, error) {
	const query = `INSERT INTO ` + pushLogTable + ` (user_id, push_type, title, content, status, biz_type, biz_id) VALUES (?, ?, ?, ?, ?, ?, ?)`
	res, err := m.conn.ExecCtx(ctx, query,
		data.UserId, data.PushType, data.Title, data.Content, data.Status, data.BizType, data.BizId)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

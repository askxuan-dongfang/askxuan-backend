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
	List(ctx context.Context, userId, status, bizType string, page, size int) ([]*PushLog, int64, error)
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

func (m *defaultPushLogModel) List(ctx context.Context, userId, status, bizType string, page, size int) ([]*PushLog, int64, error) {
	where := "1=1"
	args := make([]interface{}, 0, 5)
	if userId != "" {
		where += " AND user_id = ?"
		args = append(args, userId)
	}
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}
	if bizType != "" {
		where += " AND biz_type = ?"
		args = append(args, bizType)
	}

	var total int64
	countQuery := `SELECT COUNT(1) FROM ` + pushLogTable + ` WHERE ` + where
	if err := m.conn.QueryRowCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*PushLog{}, 0, nil
	}

	offset := (page - 1) * size
	listArgs := append(args, offset, size)
	query := `SELECT id, user_id, push_type, title, content, status, biz_type, biz_id, create_time FROM ` + pushLogTable + ` WHERE ` + where + ` ORDER BY id DESC LIMIT ?, ?`
	var list []*PushLog
	if err := m.conn.QueryRowsCtx(ctx, &list, query, listArgs...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

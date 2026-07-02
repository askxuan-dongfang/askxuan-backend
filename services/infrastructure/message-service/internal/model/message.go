package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// messageTable 消息表（位于 askxuan 库，跨库查询使用全限定名）
const messageTable = "askxuan.message"

// Message 消息实体
type Message struct {
	Id         int64  `db:"id" json:"id"`
	UserId     string `db:"user_id" json:"userId"`
	Title      string `db:"title" json:"title"`
	Content    string `db:"content" json:"content"`
	BizType    string `db:"biz_type" json:"bizType"`
	BizId      string `db:"biz_id" json:"bizId"`
	IsRead     int    `db:"is_read" json:"isRead"`
	CreateTime string `db:"create_time" json:"createTime"`
}

// MessageModel 消息模型接口
type MessageModel interface {
	List(ctx context.Context, userId string, isRead int, page, size int) ([]*Message, int64, error)
	Insert(ctx context.Context, m *Message) (int64, error)
	MarkRead(ctx context.Context, id int64) error
	UnreadCount(ctx context.Context, userId string) (int64, error)
	MarkAllRead(ctx context.Context, userId string) (int64, error)
	DeleteMessage(ctx context.Context, id int64) error
}

type defaultMessageModel struct {
	conn sqlx.SqlConn
}

// NewMessageModel 构造消息模型
func NewMessageModel(conn sqlx.SqlConn) MessageModel {
	return &defaultMessageModel{conn: conn}
}

// messageRows 消息表查询字段
const messageRows = "id, user_id, title, content, biz_type, biz_id, is_read, create_time"

// List 查询用户消息列表，isRead=-1 表示全部，按 create_time 倒序分页
func (m *defaultMessageModel) List(ctx context.Context, userId string, isRead int, page, size int) ([]*Message, int64, error) {
	where := "WHERE user_id = ?"
	args := []interface{}{userId}
	if isRead != -1 {
		where += " AND is_read = ?"
		args = append(args, isRead)
	}

	countQuery := fmt.Sprintf("SELECT COUNT(1) FROM %s %s", messageTable, where)
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*Message{}, 0, nil
	}

	offset := (page - 1) * size
	listQuery := fmt.Sprintf("SELECT %s FROM %s %s ORDER BY create_time DESC LIMIT ?, ?", messageRows, messageTable, where)
	listArgs := append(args, offset, size)
	var list []*Message
	if err := m.conn.QueryRowsCtx(ctx, &list, listQuery, listArgs...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// Insert 新建消息，返回自增 ID
func (m *defaultMessageModel) Insert(ctx context.Context, data *Message) (int64, error) {
	const query = `INSERT INTO ` + messageTable + ` (user_id, title, content, biz_type, biz_id, is_read) VALUES (?, ?, ?, ?, ?, ?)`
	res, err := m.conn.ExecCtx(ctx, query,
		data.UserId, data.Title, data.Content, data.BizType, data.BizId, data.IsRead)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// MarkRead 标记单条消息为已读
func (m *defaultMessageModel) MarkRead(ctx context.Context, id int64) error {
	const query = `UPDATE ` + messageTable + ` SET is_read = 1 WHERE id = ?`
	_, err := m.conn.ExecCtx(ctx, query, id)
	return err
}

// UnreadCount 查询用户未读消息数
func (m *defaultMessageModel) UnreadCount(ctx context.Context, userId string) (int64, error) {
	const query = `SELECT COUNT(1) FROM ` + messageTable + ` WHERE user_id = ? AND is_read = 0`
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, query, userId); err != nil {
		return 0, err
	}
	return total, nil
}

// MarkAllRead 标记用户所有未读消息为已读，返回受影响行数
func (m *defaultMessageModel) MarkAllRead(ctx context.Context, userId string) (int64, error) {
	const query = `UPDATE ` + messageTable + ` SET is_read = 1 WHERE user_id = ? AND is_read = 0`
	res, err := m.conn.ExecCtx(ctx, query, userId)
	if err != nil {
		return 0, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return affected, nil
}

// DeleteMessage 删除消息
func (m *defaultMessageModel) DeleteMessage(ctx context.Context, id int64) error {
	const query = `DELETE FROM ` + messageTable + ` WHERE id = ?`
	_, err := m.conn.ExecCtx(ctx, query, id)
	return err
}

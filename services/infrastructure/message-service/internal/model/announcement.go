package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// announcementTable 系统公告表（位于 askxuan_message 库）
const announcementTable = "system_announcement"

// 系统公告状态常量
const (
	AnnouncementStatusDraft     = "draft"
	AnnouncementStatusPublished = "published"
)

// SystemAnnouncement 系统公告实体
type SystemAnnouncement struct {
	Id             int64  `db:"id" json:"id"`
	Title          string `db:"title" json:"title"`
	Content        string `db:"content" json:"content"`
	Type           string `db:"type" json:"type"`
	TargetAudience string `db:"target_audience" json:"targetAudience"`
	Status         string `db:"status" json:"status"`
	PublishTime    string `db:"publish_time" json:"publishTime"`
	CreateTime     string `db:"create_time" json:"createTime"`
	UpdateTime     string `db:"update_time" json:"updateTime"`
}

// AnnouncementModel 系统公告模型接口
type AnnouncementModel interface {
	ListPublished(ctx context.Context, aType, audience string, page, size int) ([]*SystemAnnouncement, int64, error)
	ListAll(ctx context.Context, page, size int) ([]*SystemAnnouncement, int64, error)
	Insert(ctx context.Context, a *SystemAnnouncement) (int64, error)
	UpdateStatus(ctx context.Context, id int64, status string) error
}

type defaultAnnouncementModel struct {
	conn sqlx.SqlConn
}

// NewAnnouncementModel 构造系统公告模型
func NewAnnouncementModel(conn sqlx.SqlConn) AnnouncementModel {
	return &defaultAnnouncementModel{conn: conn}
}

// announcementRows 公告表查询字段（publish_time 使用 IFNULL 兜底）
const announcementRows = "id, title, content, type, target_audience, status, IFNULL(publish_time, '') AS publish_time, create_time, update_time"

// ListPublished 查询已发布公告列表，支持按 type/target_audience 筛选，按 publish_time 倒序
func (m *defaultAnnouncementModel) ListPublished(ctx context.Context, aType, audience string, page, size int) ([]*SystemAnnouncement, int64, error) {
	where := "WHERE status = ?"
	args := []interface{}{AnnouncementStatusPublished}
	if aType != "" {
		where += " AND type = ?"
		args = append(args, aType)
	}
	if audience != "" {
		where += " AND target_audience = ?"
		args = append(args, audience)
	}
	return m.queryPage(ctx, where, args, "ORDER BY publish_time DESC", page, size)
}

// ListAll 查询全部公告列表，按 create_time 倒序
func (m *defaultAnnouncementModel) ListAll(ctx context.Context, page, size int) ([]*SystemAnnouncement, int64, error) {
	return m.queryPage(ctx, "", []interface{}{}, "ORDER BY create_time DESC", page, size)
}

// queryPage 分页查询公告列表
func (m *defaultAnnouncementModel) queryPage(ctx context.Context, where string, args []interface{}, orderBy string, page, size int) ([]*SystemAnnouncement, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	offset := (page - 1) * size

	var total int64
	countQuery := fmt.Sprintf("SELECT COUNT(1) FROM %s %s", announcementTable, where)
	if err := m.conn.QueryRowCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*SystemAnnouncement{}, 0, nil
	}

	var list []*SystemAnnouncement
	listQuery := fmt.Sprintf("SELECT %s FROM %s %s %s LIMIT ?, ?", announcementRows, announcementTable, where, orderBy)
	listArgs := append(args, offset, size)
	if err := m.conn.QueryRowsCtx(ctx, &list, listQuery, listArgs...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// Insert 新建公告，status='published' 时设置 publish_time=NOW()
func (m *defaultAnnouncementModel) Insert(ctx context.Context, data *SystemAnnouncement) (int64, error) {
	if data.Status == AnnouncementStatusPublished {
		const query = `INSERT INTO ` + announcementTable + ` (title, content, type, target_audience, status, publish_time) VALUES (?, ?, ?, ?, ?, NOW())`
		res, err := m.conn.ExecCtx(ctx, query,
			data.Title, data.Content, data.Type, data.TargetAudience, data.Status)
		if err != nil {
			return 0, err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return 0, err
		}
		return id, nil
	}
	const query = `INSERT INTO ` + announcementTable + ` (title, content, type, target_audience, status) VALUES (?, ?, ?, ?, ?)`
	res, err := m.conn.ExecCtx(ctx, query,
		data.Title, data.Content, data.Type, data.TargetAudience, data.Status)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// UpdateStatus 更新公告状态，status='published' 时同时设置 publish_time=NOW()
func (m *defaultAnnouncementModel) UpdateStatus(ctx context.Context, id int64, status string) error {
	if status == AnnouncementStatusPublished {
		const query = `UPDATE ` + announcementTable + ` SET status = ?, publish_time = NOW() WHERE id = ?`
		_, err := m.conn.ExecCtx(ctx, query, status, id)
		return err
	}
	const query = `UPDATE ` + announcementTable + ` SET status = ? WHERE id = ?`
	_, err := m.conn.ExecCtx(ctx, query, status, id)
	return err
}

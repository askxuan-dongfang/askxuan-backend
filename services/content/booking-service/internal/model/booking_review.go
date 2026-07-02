package model

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ============ 预约评价 MySQL 存储 ============

// reviewTable 评价表（位于 askxuan_booking 库）
const reviewTable = "askxuan_booking.booking_review"

// BookingReview 预约评价
type BookingReview struct {
	Id          int64    `db:"id" json:"id"`
	BookingId   string   `db:"booking_id" json:"bookingId"`
	UserId      string   `db:"user_id" json:"userId"`
	Rating      int      `db:"rating" json:"rating"`
	Content     string   `db:"content" json:"content"`
	Images      []string `json:"images"`
	MasterReply string   `db:"master_reply" json:"masterReply"`
	CreateTime  string   `db:"create_time" json:"createTime"`
}

// bookingReviewRow 评价 DB 行结构（Images 为 JSON 字符串）
type bookingReviewRow struct {
	Id          int64  `db:"id"`
	BookingId   string `db:"booking_id"`
	UserId      string `db:"user_id"`
	Rating      int    `db:"rating"`
	Content     string `db:"content"`
	Images      string `db:"images"`
	MasterReply string `db:"master_reply"`
	CreateTime  string `db:"create_time"`
}

// BookingReviewModel 评价模型接口
type BookingReviewModel interface {
	Insert(ctx context.Context, data *BookingReview) (*BookingReview, error)
	FindOne(ctx context.Context, bookingId string) (*BookingReview, error)
	UpdateReply(ctx context.Context, bookingId, reply string) (*BookingReview, error)
}

type defaultBookingReviewModel struct {
	conn sqlx.SqlConn
}

// NewBookingReviewModel 构造评价模型
func NewBookingReviewModel(conn sqlx.SqlConn) BookingReviewModel {
	return &defaultBookingReviewModel{conn: conn}
}

// Insert 创建评价
func (m *defaultBookingReviewModel) Insert(ctx context.Context, data *BookingReview) (*BookingReview, error) {
	imagesJSON := "[]"
	if len(data.Images) > 0 {
		if b, err := json.Marshal(data.Images); err == nil {
			imagesJSON = string(b)
		}
	}
	data.CreateTime = time.Now().Format("2006-01-02 15:04:05")

	query := fmt.Sprintf(
		"INSERT INTO %s (booking_id, user_id, rating, content, images, master_reply, create_time) VALUES (?, ?, ?, ?, ?, '', ?)",
		reviewTable)
	_, err := m.conn.ExecCtx(ctx, query, data.BookingId, data.UserId, data.Rating,
		data.Content, imagesJSON, data.CreateTime)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// FindOne 按预约单号查询评价
func (m *defaultBookingReviewModel) FindOne(ctx context.Context, bookingId string) (*BookingReview, error) {
	query := fmt.Sprintf(
		"SELECT id, booking_id, user_id, rating, content, images, master_reply, create_time FROM %s WHERE booking_id = ?",
		reviewTable)
	var row bookingReviewRow
	if err := m.conn.QueryRowCtx(ctx, &row, query, bookingId); err != nil {
		return nil, err
	}
	return rowToReview(&row), nil
}

// UpdateReply 更新法师回复，返回更新后的评价
func (m *defaultBookingReviewModel) UpdateReply(ctx context.Context, bookingId, reply string) (*BookingReview, error) {
	query := fmt.Sprintf("UPDATE %s SET master_reply = ? WHERE booking_id = ?", reviewTable)
	_, err := m.conn.ExecCtx(ctx, query, reply, bookingId)
	if err != nil {
		return nil, err
	}
	return m.FindOne(ctx, bookingId)
}

// rowToReview 将 DB 行结构转为 BookingReview（Images JSON 反序列化）
func rowToReview(row *bookingReviewRow) *BookingReview {
	var images []string
	if row.Images != "" {
		_ = json.Unmarshal([]byte(row.Images), &images)
	}
	if images == nil {
		images = []string{}
	}
	return &BookingReview{
		Id:          row.Id,
		BookingId:   row.BookingId,
		UserId:      row.UserId,
		Rating:      row.Rating,
		Content:     row.Content,
		Images:      images,
		MasterReply: row.MasterReply,
		CreateTime:  row.CreateTime,
	}
}

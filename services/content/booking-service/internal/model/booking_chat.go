package model

import (
	"context"
	"errors"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const bookingChatMessageTable = "booking_chat_message"

type BookingChatMessage struct {
	Id                int64  `db:"id" json:"id"`
	BookingId         string `db:"booking_id" json:"bookingId"`
	SourceType        string `db:"source_type" json:"sourceType"`
	ClientMessageId   string `db:"client_message_id" json:"clientMessageId"`
	OpenIMServerMsgId string `db:"openim_server_msg_id" json:"openimServerMsgId"`
	SenderType        string `db:"sender_type" json:"senderType"`
	SenderId          string `db:"sender_id" json:"senderId"`
	ReceiverId        string `db:"receiver_id" json:"receiverId"`
	Content           string `db:"content" json:"content"`
	Status            string `db:"status" json:"status"`
	CreateTime        string `db:"create_time" json:"createTime"`
}

type BookingChatConversation struct {
	BookingId     string `db:"booking_id"`
	SourceType    string `db:"source_type"`
	UserId        string `db:"user_id"`
	MasterCode    string `db:"master_code"`
	MasterName    string `db:"master_name"`
	TempleName    string `db:"temple_name"`
	ServiceName   string `db:"service_name"`
	BookingDate   string `db:"booking_date"`
	ExpiresAt     string `db:"expires_at"`
	ChatStatus    string `db:"chat_status"`
	LastMessage   string `db:"last_message"`
	LastMessageAt string `db:"last_message_at"`
}

type BookingChatModel interface {
	Insert(ctx context.Context, data *BookingChatMessage) (*BookingChatMessage, bool, error)
	UpdateDelivery(ctx context.Context, id int64, status, serverMsgId string) error
	FindByClientMessageID(ctx context.Context, bookingId, clientMessageId string) (*BookingChatMessage, error)
	ListMessages(ctx context.Context, bookingId string, page, size int) ([]*BookingChatMessage, int64, error)
	ListConversations(ctx context.Context, userId, masterCode string, page, size int) ([]*BookingChatConversation, int64, error)
}

type defaultBookingChatModel struct{ conn sqlx.SqlConn }

func NewBookingChatModel(conn sqlx.SqlConn) BookingChatModel {
	return &defaultBookingChatModel{conn: conn}
}

const bookingChatSelect = `id,booking_id,source_type,client_message_id,openim_server_msg_id,sender_type,sender_id,receiver_id,content,status,DATE_FORMAT(create_time,'%Y-%m-%d %H:%i:%s') create_time`

func (m *defaultBookingChatModel) Insert(ctx context.Context, data *BookingChatMessage) (*BookingChatMessage, bool, error) {
	if existing, err := m.FindByClientMessageID(ctx, data.BookingId, data.ClientMessageId); err == nil {
		return existing, false, nil
	} else if !errors.Is(err, sqlx.ErrNotFound) {
		return nil, false, err
	}
	if data.Status == "" {
		data.Status = "pending"
	}
	if data.SourceType == "" {
		data.SourceType = "booking"
	}
	result, err := m.conn.ExecCtx(ctx, `INSERT INTO `+bookingChatMessageTable+` (booking_id,source_type,client_message_id,openim_server_msg_id,sender_type,sender_id,receiver_id,content,status,create_time) VALUES(?,?,?,?,?,?,?,?,?,NOW())`,
		data.BookingId, data.SourceType, data.ClientMessageId, data.OpenIMServerMsgId, data.SenderType, data.SenderId, data.ReceiverId, data.Content, data.Status)
	if err != nil {
		if existing, findErr := m.FindByClientMessageID(ctx, data.BookingId, data.ClientMessageId); findErr == nil {
			return existing, false, nil
		}
		return nil, false, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, false, err
	}
	data.Id = id
	data.CreateTime = time.Now().Format("2006-01-02 15:04:05")
	return data, true, nil
}

func (m *defaultBookingChatModel) UpdateDelivery(ctx context.Context, id int64, status, serverMsgId string) error {
	_, err := m.conn.ExecCtx(ctx, `UPDATE `+bookingChatMessageTable+` SET status=?,openim_server_msg_id=IF(?='',openim_server_msg_id,?) WHERE id=?`, status, serverMsgId, serverMsgId, id)
	return err
}

func (m *defaultBookingChatModel) FindByClientMessageID(ctx context.Context, bookingId, clientMessageId string) (*BookingChatMessage, error) {
	var message BookingChatMessage
	err := m.conn.QueryRowCtx(ctx, &message, `SELECT `+bookingChatSelect+` FROM `+bookingChatMessageTable+` WHERE booking_id=? AND client_message_id=?`, bookingId, clientMessageId)
	return &message, err
}

func (m *defaultBookingChatModel) ListMessages(ctx context.Context, bookingId string, page, size int) ([]*BookingChatMessage, int64, error) {
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, `SELECT COUNT(1) FROM `+bookingChatMessageTable+` WHERE booking_id=? AND status='sent'`, bookingId); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*BookingChatMessage{}, 0, nil
	}
	var list []*BookingChatMessage
	// Fetch pages from the newest message backwards, then present each page chronologically.
	err := m.conn.QueryRowsCtx(ctx, &list, `SELECT `+bookingChatSelect+` FROM `+bookingChatMessageTable+` WHERE id IN (SELECT id FROM (SELECT id FROM `+bookingChatMessageTable+` WHERE booking_id=? AND status='sent' ORDER BY id DESC LIMIT ?,?) latest) ORDER BY id ASC`, bookingId, (page-1)*size, size)
	return list, total, err
}

func (m *defaultBookingChatModel) ListConversations(ctx context.Context, userId, masterCode string, page, size int) ([]*BookingChatConversation, int64, error) {
	where := `x.payment_status='success' AND x.chat_status<>'closed'`
	args := []interface{}{}
	if masterCode != "" {
		where += ` AND x.master_code=?`
		args = append(args, masterCode)
	} else {
		where += ` AND x.user_id=?`
		args = append(args, userId)
	}
	const sources = `(SELECT b.booking_no source_id,'booking' source_type,b.user_id,b.master_code,b.master_name,b.temple_name,b.service_name,DATE_FORMAT(b.booking_date,'%Y-%m-%d') booking_date,'' expires_at,b.payment_status,IF(b.status<>'cancelled','active','closed') chat_status,b.create_time
		FROM booking b
		UNION ALL
		SELECT c.order_no source_id,'consultation' source_type,c.user_id,c.master_code,c.master_name,c.temple_name,'即时文字咨询' service_name,'' booking_date,COALESCE(DATE_FORMAT(c.expires_at,'%Y-%m-%d %H:%i:%s'),'') expires_at,c.payment_status,CASE WHEN c.status='active' AND c.expires_at>NOW() THEN 'active' WHEN c.status IN ('active','expired','closed') THEN 'expired' ELSE 'closed' END chat_status,c.create_time
		FROM consultation_order c) x`
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, `SELECT COUNT(1) FROM `+sources+` WHERE `+where, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*BookingChatConversation{}, 0, nil
	}
	queryArgs := append(append([]interface{}{}, args...), (page-1)*size, size)
	const columns = `x.source_id booking_id,x.source_type,x.user_id,x.master_code,x.master_name,x.temple_name,x.service_name,x.booking_date,x.expires_at,x.chat_status,COALESCE(m.content,'付款成功，可开始对话') last_message,COALESCE(DATE_FORMAT(m.create_time,'%Y-%m-%d %H:%i:%s'),DATE_FORMAT(x.create_time,'%Y-%m-%d %H:%i:%s')) last_message_at`
	query := `SELECT ` + columns + ` FROM ` + sources + ` LEFT JOIN booking_chat_message m ON m.id=(SELECT MAX(m2.id) FROM booking_chat_message m2 WHERE m2.booking_id=x.source_id AND m2.source_type=x.source_type AND m2.status='sent') WHERE ` + where + ` ORDER BY COALESCE(m.create_time,x.create_time) DESC LIMIT ?,?`
	var list []*BookingChatConversation
	if err := m.conn.QueryRowsCtx(ctx, &list, query, queryArgs...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

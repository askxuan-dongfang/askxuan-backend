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
	Kind              string `db:"kind"`
	AttachmentJSON    string `db:"attachment_json"`
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
	MessagesCursor(context.Context, string, int64, int64, int) ([]*BookingChatMessage, bool, error)
	ReadCursor(context.Context, string, string) (int64, error)
	MarkRead(context.Context, string, string, int64) (int64, error)
	Unread(context.Context, string, string) (int64, error)

	Insert(ctx context.Context, data *BookingChatMessage) (*BookingChatMessage, bool, error)
	UpdateDelivery(ctx context.Context, id int64, status, serverMsgId string) error
	FindByClientMessageID(ctx context.Context, bookingId, clientMessageId string) (*BookingChatMessage, error)
	ListMessages(ctx context.Context, bookingId string, page, size int) ([]*BookingChatMessage, int64, error)
	ListConversations(ctx context.Context, userId, masterCode string, page, size int, queryText, reader string, unreadOnly bool) ([]*BookingChatConversation, int64, error)
}

type defaultBookingChatModel struct{ conn sqlx.SqlConn }

func NewBookingChatModel(conn sqlx.SqlConn) BookingChatModel {
	return &defaultBookingChatModel{conn: conn}
}

const bookingChatSelect = `id,booking_id,source_type,client_message_id,openim_server_msg_id,sender_type,sender_id,receiver_id,content,kind,attachment_json,status,DATE_FORMAT(create_time,'%Y-%m-%d %H:%i:%s') create_time`

func (m *defaultBookingChatModel) Insert(ctx context.Context, data *BookingChatMessage) (*BookingChatMessage, bool, error) {
	if existing, err := m.FindByClientMessageID(ctx, data.BookingId, data.ClientMessageId); err == nil {
		return existing, false, nil
	} else if !errors.Is(err, sqlx.ErrNotFound) {
		return nil, false, err
	}
	if data.Kind == "" {
		data.Kind = "text"
	}
	if data.AttachmentJSON == "" {
		data.AttachmentJSON = "{}"
	}
	if data.Status == "" {
		data.Status = "pending"
	}
	if data.SourceType == "" {
		data.SourceType = "booking"
	}
	result, err := m.conn.ExecCtx(ctx, `INSERT INTO `+bookingChatMessageTable+` (booking_id,source_type,client_message_id,openim_server_msg_id,sender_type,sender_id,receiver_id,content,kind,attachment_json,status,notify_status,notify_after,push_status,push_after,create_time) VALUES(?,?,?,?,?,?,?,?,?,?,?,'pending',NOW(),'pending',NOW(),NOW())`,
		data.BookingId, data.SourceType, data.ClientMessageId, data.OpenIMServerMsgId, data.SenderType, data.SenderId, data.ReceiverId, data.Content, data.Kind, data.AttachmentJSON, data.Status)
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

func (m *defaultBookingChatModel) ListConversations(ctx context.Context, userId, masterCode string, page, size int, queryText, reader string, unreadOnly bool) ([]*BookingChatConversation, int64, error) {
	where := `x.payment_status='success' AND x.chat_status<>'closed'`
	args := []interface{}{}
	if masterCode != "" {
		where += ` AND x.master_code=?`
		args = append(args, masterCode)
	} else {
		where += ` AND x.user_id=?`
		args = append(args, userId)
	}
	if queryText != "" {
		where += ` AND (x.master_name LIKE ? OR x.service_name LIKE ? OR EXISTS(SELECT 1 FROM askxuan_user.user u WHERE CAST(u.id AS CHAR)=x.user_id AND u.nickname LIKE ?) OR EXISTS(SELECT 1 FROM booking_chat_message cm WHERE cm.booking_id=x.source_id AND cm.status='sent' AND cm.content LIKE ?))`
		term := "%" + queryText + "%"
		args = append(args, term, term, term, term)
	}
	if unreadOnly {
		where += ` AND EXISTS(SELECT 1 FROM booking_chat_message cm WHERE cm.booking_id=x.source_id AND cm.receiver_id=? AND cm.status='sent' AND cm.id>COALESCE((SELECT through_id FROM chat_read_cursor cr WHERE cr.conversation_id=x.source_id AND cr.reader_id=?),0))`
		args = append(args, reader, reader)
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

// Stable keyset pagination avoids skipped/duplicated rows when new messages arrive.
func (m *defaultBookingChatModel) MessagesCursor(ctx context.Context, id string, before, after int64, size int) ([]*BookingChatMessage, bool, error) {
	where := "booking_id=? AND status='sent'"
	args := []interface{}{id}
	order := "DESC"
	if before > 0 {
		where += " AND id<?"
		args = append(args, before)
	}
	if after > 0 {
		where += " AND id>?"
		args = append(args, after)
		order = "ASC"
	}
	args = append(args, size+1)
	var rows []*BookingChatMessage
	err := m.conn.QueryRowsCtx(ctx, &rows, "SELECT "+bookingChatSelect+" FROM booking_chat_message WHERE "+where+" ORDER BY id "+order+" LIMIT ?", args...)
	more := len(rows) > size
	if more {
		rows = rows[:size]
	}
	if order == "DESC" {
		for i, j := 0, len(rows)-1; i < j; i, j = i+1, j-1 {
			rows[i], rows[j] = rows[j], rows[i]
		}
	}
	if rows == nil {
		rows = []*BookingChatMessage{}
	}
	return rows, more, err
}
func (m *defaultBookingChatModel) ReadCursor(ctx context.Context, id, reader string) (int64, error) {
	var n int64
	err := m.conn.QueryRowCtx(ctx, &n, "SELECT COALESCE(MAX(through_id),0) FROM chat_read_cursor WHERE conversation_id=? AND reader_id=?", id, reader)
	return n, err
}
func (m *defaultBookingChatModel) MarkRead(ctx context.Context, id, reader string, through int64) (int64, error) {
	// Clamp to a message that the client actually fetched, never a future/global ID.
	var valid int64
	if err := m.conn.QueryRowCtx(ctx, &valid, "SELECT COALESCE(MAX(id),0) FROM booking_chat_message WHERE booking_id=? AND status='sent' AND id<=?", id, through); err != nil {
		return 0, err
	}
	_, err := m.conn.ExecCtx(ctx, "INSERT INTO chat_read_cursor(conversation_id,reader_id,through_id) VALUES(?,?,?) ON DUPLICATE KEY UPDATE through_id=GREATEST(through_id,VALUES(through_id)),updated_at=NOW()", id, reader, valid)
	if err != nil {
		return 0, err
	}
	return m.ReadCursor(ctx, id, reader)
}
func (m *defaultBookingChatModel) Unread(ctx context.Context, id, reader string) (int64, error) {
	var n int64
	err := m.conn.QueryRowCtx(ctx, &n, "SELECT COUNT(*) FROM booking_chat_message WHERE booking_id=? AND receiver_id=? AND status='sent' AND id>COALESCE((SELECT through_id FROM chat_read_cursor WHERE conversation_id=? AND reader_id=?),0)", id, reader, id, reader)
	return n, err
}

package model

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	ConsultationPaymentPending       = "pending"
	ConsultationPaymentSuccess       = "success"
	ConsultationStatusPendingPayment = "pending_payment"
	ConsultationStatusActive         = "active"
	ConsultationStatusExpired        = "expired"
	ConsultationStatusClosed         = "closed"
)

type Consultation struct {
	Id              string  `db:"order_no" json:"id"`
	RequestId       string  `db:"request_id" json:"requestId"`
	UserId          string  `db:"user_id" json:"userId"`
	MasterId        string  `db:"master_code" json:"masterId"`
	MasterName      string  `db:"master_name" json:"masterName"`
	TempleId        string  `db:"temple_code" json:"templeId"`
	TempleName      string  `db:"temple_name" json:"templeName"`
	ConsultFee      float64 `db:"consult_fee" json:"consultFee"`
	ValidHours      int     `db:"valid_hours" json:"validHours"`
	ResponseMinutes int     `db:"response_minutes" json:"responseMinutes"`
	Question        string  `db:"question" json:"question"`
	PriceSnapshot   string  `db:"price_snapshot" json:"priceSnapshot"`
	PaymentNo       string  `db:"payment_no" json:"paymentNo"`
	PaymentChannel  string  `db:"payment_channel" json:"paymentChannel"`
	PaymentStatus   string  `db:"payment_status" json:"paymentStatus"`
	Status          string  `db:"status" json:"status"`
	ValidFrom       string  `db:"valid_from" json:"validFrom"`
	ExpiresAt       string  `db:"expires_at" json:"expiresAt"`
	CreateTime      string  `db:"create_time" json:"createTime"`
}

type ConsultationModel interface {
	Insert(ctx context.Context, data *Consultation) (*Consultation, error)
	FindOne(ctx context.Context, id string) (*Consultation, error)
	FindByRequest(ctx context.Context, userID, requestID string) (*Consultation, error)
	Activate(ctx context.Context, id, paymentNo, channel string) (*Consultation, bool, error)
	List(ctx context.Context, userID, masterCode, status string, page, size int) ([]*Consultation, int64, error)
	FindPendingPayments(ctx context.Context, limit int) ([]*Consultation, error)
	ExpireActive(ctx context.Context) (int64, error)
}

type defaultConsultationModel struct{ conn sqlx.SqlConn }

func NewConsultationModel(conn sqlx.SqlConn) ConsultationModel {
	return &defaultConsultationModel{conn: conn}
}

const consultationSelect = `order_no,request_id,user_id,master_code,master_name,temple_code,temple_name,consult_fee,valid_hours,response_minutes,question,COALESCE(CAST(price_snapshot AS CHAR),'') price_snapshot,payment_no,payment_channel,payment_status,status,COALESCE(DATE_FORMAT(valid_from,'%Y-%m-%d %H:%i:%s'),'') valid_from,COALESCE(DATE_FORMAT(expires_at,'%Y-%m-%d %H:%i:%s'),'') expires_at,DATE_FORMAT(create_time,'%Y-%m-%d %H:%i:%s') create_time`

func (m *defaultConsultationModel) Insert(ctx context.Context, data *Consultation) (*Consultation, error) {
	if data.Id == "" {
		data.Id = "C" + time.Now().Format("20060102150405") + fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	}
	_, err := m.conn.ExecCtx(ctx, `INSERT INTO consultation_order
		(order_no,request_id,user_id,master_code,master_name,temple_code,temple_name,consult_fee,valid_hours,response_minutes,question,price_snapshot,payment_status,status)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,CAST(? AS JSON),'pending','pending_payment')`, data.Id, data.RequestId, data.UserId,
		data.MasterId, data.MasterName, data.TempleId, data.TempleName, data.ConsultFee, data.ValidHours,
		data.ResponseMinutes, data.Question, data.PriceSnapshot)
	if err != nil {
		return nil, err
	}
	return m.FindOne(ctx, data.Id)
}

func (m *defaultConsultationModel) FindOne(ctx context.Context, id string) (*Consultation, error) {
	var row Consultation
	err := m.conn.QueryRowCtx(ctx, &row, `SELECT `+consultationSelect+` FROM consultation_order WHERE order_no=?`, id)
	return &row, err
}

func (m *defaultConsultationModel) FindByRequest(ctx context.Context, userID, requestID string) (*Consultation, error) {
	var row Consultation
	err := m.conn.QueryRowCtx(ctx, &row, `SELECT `+consultationSelect+` FROM consultation_order WHERE user_id=? AND request_id=?`, userID, requestID)
	return &row, err
}

func (m *defaultConsultationModel) Activate(ctx context.Context, id, paymentNo, channel string) (*Consultation, bool, error) {
	result, err := m.conn.ExecCtx(ctx, `UPDATE consultation_order SET payment_no=?,payment_channel=?,payment_status='success',status='active',valid_from=COALESCE(valid_from,NOW()),expires_at=COALESCE(expires_at,DATE_ADD(NOW(),INTERVAL valid_hours HOUR)) WHERE order_no=? AND status='pending_payment'`, paymentNo, channel, id)
	if err != nil {
		return nil, false, err
	}
	rows, _ := result.RowsAffected()
	row, err := m.FindOne(ctx, id)
	return row, rows > 0, err
}

func (m *defaultConsultationModel) List(ctx context.Context, userID, masterCode, status string, page, size int) ([]*Consultation, int64, error) {
	where := "WHERE 1=1"
	args := make([]interface{}, 0, 3)
	if userID != "" {
		where += " AND user_id=?"
		args = append(args, userID)
	}
	if masterCode != "" {
		where += " AND master_code=?"
		args = append(args, masterCode)
	}
	if status != "" {
		where += " AND status=?"
		args = append(args, status)
	}
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, `SELECT COUNT(1) FROM consultation_order `+where, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*Consultation{}, 0, nil
	}
	queryArgs := append(append([]interface{}{}, args...), (page-1)*size, size)
	var rows []*Consultation
	err := m.conn.QueryRowsCtx(ctx, &rows, `SELECT `+consultationSelect+` FROM consultation_order `+where+` ORDER BY create_time DESC LIMIT ?,?`, queryArgs...)
	return rows, total, err
}

func (m *defaultConsultationModel) FindPendingPayments(ctx context.Context, limit int) ([]*Consultation, error) {
	var rows []*Consultation
	err := m.conn.QueryRowsCtx(ctx, &rows, `SELECT `+consultationSelect+` FROM consultation_order WHERE status='pending_payment' ORDER BY create_time LIMIT ?`, limit)
	return rows, err
}

func (m *defaultConsultationModel) ExpireActive(ctx context.Context) (int64, error) {
	result, err := m.conn.ExecCtx(ctx, `UPDATE consultation_order SET status='expired' WHERE status='active' AND expires_at<=NOW()`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

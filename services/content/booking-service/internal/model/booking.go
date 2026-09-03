package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	StatusPendingPayment = "pending_payment"
	StatusPending        = "pending"
	StatusConfirmed      = "confirmed"
	StatusInProgress     = "in_progress"
	StatusCompleted      = "completed"
	StatusCancelled      = "cancelled"
	StatusReviewed       = "reviewed"

	PaymentStatusPending = "pending"
	PaymentStatusSuccess = "success"
	PaymentStatusFailed  = "failed"
	PaymentStatusLegacy  = "legacy"
)

var (
	ErrSlotFull         = errors.New("booking slot full")
	ErrPaymentExpired   = errors.New("booking payment expired")
	ErrRequestDuplicate = errors.New("booking request duplicate")
)

var validTransitions = map[string]map[string]bool{
	StatusPendingPayment: {StatusPending: true, StatusCancelled: true},
	StatusPending:        {StatusConfirmed: true, StatusCancelled: true},
	StatusConfirmed:      {StatusInProgress: true, StatusCancelled: true},
	StatusInProgress:     {StatusCompleted: true, StatusCancelled: true},
	StatusCompleted:      {StatusReviewed: true},
}

func CanTransit(from, to string) bool { return from != to && validTransitions[from][to] }
func IsTerminalStatus(s string) bool  { return s == StatusReviewed || s == StatusCancelled }

type Booking struct {
	Id                string  `db:"booking_no" json:"id"`
	RequestId         string  `db:"request_id" json:"requestId"`
	UserId            string  `db:"user_id" json:"userId"`
	TempleId          string  `db:"temple_code" json:"templeId"`
	TempleName        string  `db:"temple_name" json:"templeName"`
	MasterId          string  `db:"master_code" json:"masterId"`
	MasterName        string  `db:"master_name" json:"masterName"`
	ServiceId         string  `db:"service_code" json:"serviceId"`
	ServiceName       string  `db:"service_name" json:"serviceName"`
	BookingDate       string  `db:"booking_date" json:"bookingDate"`
	SlotCode          string  `db:"slot_code" json:"slotCode"`
	TimeSlot          string  `db:"time_slot" json:"timeSlot"`
	ServiceFee        float64 `db:"service_fee" json:"serviceFee"`
	MeritMoney        float64 `db:"merit_money" json:"meritMoney"`
	MeritMoneyTier    string  `db:"merit_money_tier" json:"meritMoneyTier"`
	TotalFee          float64 `db:"total_fee" json:"totalFee"`
	PriceSnapshot     string  `db:"price_snapshot" json:"priceSnapshot"`
	PaymentNo         string  `db:"payment_no" json:"paymentNo"`
	PaymentChannel    string  `db:"payment_channel" json:"paymentChannel"`
	PaymentStatus     string  `db:"payment_status" json:"paymentStatus"`
	PaymentExpireTime string  `db:"payment_expire_time" json:"paymentExpireTime"`
	SlotReserved      int     `db:"slot_reserved" json:"-"`
	Status            string  `db:"status" json:"status"`
	Note              string  `db:"note" json:"note"`
	CreatedAt         string  `db:"create_time" json:"createdAt"`
}

type SlotAvailability struct {
	SlotCode      string `db:"slot_code"`
	ReservedCount int    `db:"reserved_count"`
}

type BookingModel interface {
	InsertWithReservation(ctx context.Context, data *Booking, capacity int) (*Booking, error)
	InsertDirect(ctx context.Context, data *Booking) (*Booking, error)
	FindByRequest(ctx context.Context, userId, requestId string) (*Booking, error)
	FindOne(ctx context.Context, bookingNo string) (*Booking, error)
	FindList(ctx context.Context, userId, status, templeId string, page, size int) ([]*Booking, int64, error)
	UpdateStatus(ctx context.Context, bookingNo, newStatus string) (*Booking, error)
	UpdatePayment(ctx context.Context, bookingNo, paymentNo, channel, paymentStatus, status string) (*Booking, bool, error)
	FindAdminList(ctx context.Context, templeId, status, masterId string, page, size int) ([]*Booking, int64, error)
	FindSlotUsage(ctx context.Context, templeCode, serviceCode, date string) (map[string]int, error)
	FindPendingPayments(ctx context.Context, limit int) ([]*Booking, error)
	ExpirePendingPayments(ctx context.Context) (int64, error)
	Report(ctx context.Context, templeID, start, end string) (BookingReportStats, []*BookingReportTrend, []*BookingReportService, []*BookingReportMaster, error)
}

type defaultBookingModel struct{ conn sqlx.SqlConn }

func NewBookingModel(conn sqlx.SqlConn) BookingModel { return &defaultBookingModel{conn: conn} }

const bookingSelect = `booking_no,COALESCE(request_id,'') request_id,user_id,temple_code,temple_name,master_code,master_name,service_code,service_name,DATE_FORMAT(booking_date,'%Y-%m-%d') booking_date,slot_code,time_slot,service_fee,merit_money,merit_money_tier,total_fee,COALESCE(CAST(price_snapshot AS CHAR),'') price_snapshot,payment_no,payment_channel,payment_status,COALESCE(DATE_FORMAT(payment_expire_time,'%Y-%m-%d %H:%i:%s'),'') payment_expire_time,slot_reserved,status,note,DATE_FORMAT(create_time,'%Y-%m-%d %H:%i:%s') create_time`

// InsertDirect 大师直约：不占寺院时段库存（野生大师或直连寺庙大师）
func (m *defaultBookingModel) InsertDirect(ctx context.Context, data *Booking) (*Booking, error) {
	now := time.Now()
	data.Id = fmt.Sprintf("B%s%06d", now.Format("20060102150405"), now.Nanosecond()/1e3%1000000)
	data.Status = StatusPendingPayment
	data.PaymentStatus = PaymentStatusPending
	data.PaymentExpireTime = now.Add(15 * time.Minute).Format("2006-01-02 15:04:05")
	data.CreatedAt = now.Format("2006-01-02 15:04:05")
	const insert = `INSERT INTO booking(booking_no,request_id,user_id,temple_code,temple_name,master_code,master_name,service_code,service_name,booking_date,slot_code,time_slot,service_fee,merit_money,merit_money_tier,total_fee,price_snapshot,payment_no,payment_channel,payment_status,payment_expire_time,slot_reserved,status,note,create_time) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,CAST(? AS JSON),'','','pending',?,0,?,?,?)`
	_, err := m.conn.ExecCtx(ctx, insert, data.Id, nullString(data.RequestId), data.UserId, data.TempleId, data.TempleName, data.MasterId, data.MasterName, data.ServiceId, data.ServiceName, data.BookingDate, data.SlotCode, data.TimeSlot, data.ServiceFee, data.MeritMoney, data.MeritMoneyTier, data.TotalFee, data.PriceSnapshot, data.PaymentExpireTime, data.Status, data.Note, data.CreatedAt)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (m *defaultBookingModel) InsertWithReservation(ctx context.Context, data *Booking, capacity int) (*Booking, error) {
	if capacity < 1 {
		return nil, ErrSlotFull
	}
	now := time.Now()
	data.Id = fmt.Sprintf("B%s%06d", now.Format("20060102150405"), now.Nanosecond()/1e3%1000000)
	data.Status = StatusPendingPayment
	data.PaymentStatus = PaymentStatusPending
	data.PaymentExpireTime = now.Add(15 * time.Minute).Format("2006-01-02 15:04:05")
	data.CreatedAt = now.Format("2006-01-02 15:04:05")
	err := m.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		const ensure = `INSERT INTO booking_slot_inventory(temple_code,service_code,booking_date,slot_code,time_slot,capacity,reserved_count) VALUES(?,?,?,?,?,?,0) ON DUPLICATE KEY UPDATE capacity=VALUES(capacity),time_slot=VALUES(time_slot)`
		if _, err := session.ExecCtx(ctx, ensure, data.TempleId, data.ServiceId, data.BookingDate, data.SlotCode, data.TimeSlot, capacity); err != nil {
			return err
		}
		res, err := session.ExecCtx(ctx, `UPDATE booking_slot_inventory SET reserved_count=reserved_count+1 WHERE temple_code=? AND service_code=? AND booking_date=? AND slot_code=? AND reserved_count<capacity`, data.TempleId, data.ServiceId, data.BookingDate, data.SlotCode)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n != 1 {
			return ErrSlotFull
		}
		const insert = `INSERT INTO booking(booking_no,request_id,user_id,temple_code,temple_name,master_code,master_name,service_code,service_name,booking_date,slot_code,time_slot,service_fee,merit_money,merit_money_tier,total_fee,price_snapshot,payment_no,payment_channel,payment_status,payment_expire_time,slot_reserved,status,note,create_time) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,CAST(? AS JSON),'','','pending',?,1,?,?,?)`
		_, err = session.ExecCtx(ctx, insert, data.Id, nullString(data.RequestId), data.UserId, data.TempleId, data.TempleName, data.MasterId, data.MasterName, data.ServiceId, data.ServiceName, data.BookingDate, data.SlotCode, data.TimeSlot, data.ServiceFee, data.MeritMoney, data.MeritMoneyTier, data.TotalFee, data.PriceSnapshot, data.PaymentExpireTime, data.Status, data.Note, data.CreatedAt)
		return err
	})
	if err != nil {
		return nil, err
	}
	data.SlotReserved = 1
	return data, nil
}

func nullString(value string) interface{} {
	if value == "" {
		return nil
	}
	return value
}

func (m *defaultBookingModel) FindByRequest(ctx context.Context, userId, requestId string) (*Booking, error) {
	var b Booking
	err := m.conn.QueryRowCtx(ctx, &b, `SELECT `+bookingSelect+` FROM booking WHERE user_id=? AND request_id=?`, userId, requestId)
	return &b, err
}

func (m *defaultBookingModel) FindOne(ctx context.Context, bookingNo string) (*Booking, error) {
	var b Booking
	err := m.conn.QueryRowCtx(ctx, &b, `SELECT `+bookingSelect+` FROM booking WHERE booking_no=?`, bookingNo)
	return &b, err
}

func (m *defaultBookingModel) FindList(ctx context.Context, userId, status, templeId string, page, size int) ([]*Booking, int64, error) {
	return m.findList(ctx, userId, status, templeId, "", page, size)
}
func (m *defaultBookingModel) FindAdminList(ctx context.Context, templeId, status, masterId string, page, size int) ([]*Booking, int64, error) {
	return m.findList(ctx, "", status, templeId, masterId, page, size)
}
func (m *defaultBookingModel) findList(ctx context.Context, userId, status, templeId, masterId string, page, size int) ([]*Booking, int64, error) {
	where, args := buildWhere(userId, status, templeId, masterId)
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, `SELECT COUNT(1) FROM booking WHERE `+where, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*Booking{}, 0, nil
	}
	args = append(args, (page-1)*size, size)
	var list []*Booking
	if err := m.conn.QueryRowsCtx(ctx, &list, `SELECT `+bookingSelect+` FROM booking WHERE `+where+` ORDER BY create_time DESC LIMIT ?,?`, args...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *defaultBookingModel) UpdateStatus(ctx context.Context, bookingNo, newStatus string) (*Booking, error) {
	err := m.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		var b struct {
			TempleId     string `db:"temple_code"`
			ServiceId    string `db:"service_code"`
			BookingDate  string `db:"booking_date"`
			SlotCode     string `db:"slot_code"`
			SlotReserved int    `db:"slot_reserved"`
		}
		if err := session.QueryRowCtx(ctx, &b, `SELECT temple_code,service_code,booking_date,slot_code,slot_reserved FROM booking WHERE booking_no=? FOR UPDATE`, bookingNo); err != nil {
			return err
		}
		if newStatus == StatusCancelled && b.SlotReserved == 1 {
			if _, err := session.ExecCtx(ctx, `UPDATE booking_slot_inventory SET reserved_count=GREATEST(reserved_count-1,0) WHERE temple_code=? AND service_code=? AND booking_date=? AND slot_code=?`, b.TempleId, b.ServiceId, b.BookingDate, b.SlotCode); err != nil {
				return err
			}
			_, err := session.ExecCtx(ctx, `UPDATE booking SET status=?,slot_reserved=0 WHERE booking_no=?`, newStatus, bookingNo)
			return err
		}
		_, err := session.ExecCtx(ctx, `UPDATE booking SET status=? WHERE booking_no=?`, newStatus, bookingNo)
		return err
	})
	if err != nil {
		return nil, err
	}
	return m.FindOne(ctx, bookingNo)
}

func (m *defaultBookingModel) UpdatePayment(ctx context.Context, bookingNo, paymentNo, channel, paymentStatus, newStatus string) (*Booking, bool, error) {
	res, err := m.conn.ExecCtx(ctx, `UPDATE booking SET payment_no=?,payment_channel=?,payment_status=?,status=? WHERE booking_no=? AND status='pending_payment'`, paymentNo, channel, paymentStatus, newStatus, bookingNo)
	if err != nil {
		return nil, false, err
	}
	changed, _ := res.RowsAffected()
	booking, err := m.FindOne(ctx, bookingNo)
	if err != nil {
		return nil, false, err
	}
	return booking, changed == 1, nil
}

func (m *defaultBookingModel) FindSlotUsage(ctx context.Context, templeCode, serviceCode, date string) (map[string]int, error) {
	var rows []SlotAvailability
	if err := m.conn.QueryRowsCtx(ctx, &rows, `SELECT slot_code,reserved_count FROM booking_slot_inventory WHERE temple_code=? AND service_code=? AND booking_date=?`, templeCode, serviceCode, date); err != nil {
		return nil, err
	}
	out := map[string]int{}
	for _, row := range rows {
		out[row.SlotCode] = row.ReservedCount
	}
	return out, nil
}

func (m *defaultBookingModel) FindPendingPayments(ctx context.Context, limit int) ([]*Booking, error) {
	var rows []*Booking
	err := m.conn.QueryRowsCtx(ctx, &rows, `SELECT `+bookingSelect+` FROM booking WHERE status='pending_payment' AND payment_expire_time>NOW() ORDER BY create_time LIMIT ?`, limit)
	return rows, err
}

func (m *defaultBookingModel) ExpirePendingPayments(ctx context.Context) (int64, error) {
	var ids []struct {
		Id string `db:"booking_no"`
	}
	if err := m.conn.QueryRowsCtx(ctx, &ids, `SELECT booking_no FROM booking WHERE status='pending_payment' AND payment_expire_time<=NOW()`); err != nil {
		return 0, err
	}
	var count int64
	for _, id := range ids {
		if _, err := m.UpdateStatus(ctx, id.Id, StatusCancelled); err == nil {
			count++
		}
	}
	return count, nil
}

func buildWhere(userId, status, templeId, masterId string) (string, []interface{}) {
	where := "1=1"
	args := []interface{}{}
	if userId != "" {
		where += " AND user_id=?"
		args = append(args, userId)
	}
	if status != "" {
		where += " AND status=?"
		args = append(args, status)
	}
	if templeId != "" {
		where += " AND temple_code=?"
		args = append(args, templeId)
	}
	if masterId != "" {
		where += " AND master_code=?"
		args = append(args, masterId)
	}
	return where, args
}

package model

import (
	"context"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// PlatformServiceOrderQuery is used only by the explicitly platform-scoped read API.
// The provider/customer list methods retain their existing ownership requirements.
type PlatformServiceOrderQuery struct {
	Q, Status, PaymentStatus, TempleID, MasterID, From, To string
	Page, Size                                             int
}

type PlatformServiceOrderReader interface {
	Bookings(context.Context, PlatformServiceOrderQuery) ([]*Booking, int64, error)
	Consultations(context.Context, PlatformServiceOrderQuery) ([]*Consultation, int64, error)
}

type platformServiceOrderReader struct{ conn sqlx.SqlConn }

func NewPlatformServiceOrderReader(conn sqlx.SqlConn) PlatformServiceOrderReader {
	return &platformServiceOrderReader{conn: conn}
}

func (r *platformServiceOrderReader) Bookings(ctx context.Context, q PlatformServiceOrderQuery) ([]*Booking, int64, error) {
	where, args := platformServiceOrderWhere(q, false)
	var total int64
	if err := r.conn.QueryRowCtx(ctx, &total, "SELECT COUNT(1) FROM booking WHERE "+where, args...); err != nil {
		return nil, 0, err
	}
	rows := []*Booking{}
	if total == 0 {
		return rows, total, nil
	}
	listArgs := append(append([]interface{}{}, args...), (q.Page-1)*q.Size, q.Size)
	err := r.conn.QueryRowsCtx(ctx, &rows, "SELECT "+bookingSelect+" FROM booking WHERE "+where+" ORDER BY create_time DESC,id DESC LIMIT ?,?", listArgs...)
	return rows, total, err
}

func (r *platformServiceOrderReader) Consultations(ctx context.Context, q PlatformServiceOrderQuery) ([]*Consultation, int64, error) {
	where, args := platformServiceOrderWhere(q, true)
	var total int64
	if err := r.conn.QueryRowCtx(ctx, &total, "SELECT COUNT(1) FROM consultation_order WHERE "+where, args...); err != nil {
		return nil, 0, err
	}
	rows := []*Consultation{}
	if total == 0 {
		return rows, total, nil
	}
	listArgs := append(append([]interface{}{}, args...), (q.Page-1)*q.Size, q.Size)
	err := r.conn.QueryRowsCtx(ctx, &rows, "SELECT "+consultationSelect+" FROM consultation_order WHERE "+where+" ORDER BY create_time DESC,order_no DESC LIMIT ?,?", listArgs...)
	return rows, total, err
}

// Both COUNT and page queries use this same predicate and bound arguments.
// Search is literal substring matching, including when the operator types % or _.
func platformServiceOrderWhere(q PlatformServiceOrderQuery, consultation bool) (string, []interface{}) {
	parts := []string{"1=1"}
	args := []interface{}{}
	for _, filter := range []struct{ column, value string }{
		{"status", q.Status}, {"payment_status", q.PaymentStatus},
		{"temple_code", q.TempleID}, {"master_code", q.MasterID},
	} {
		if filter.value != "" {
			parts = append(parts, filter.column+"=?")
			args = append(args, filter.value)
		}
	}
	if q.From != "" {
		parts = append(parts, "create_time>=?")
		args = append(args, q.From+" 00:00:00")
	}
	if q.To != "" {
		parts = append(parts, "create_time<DATE_ADD(?,INTERVAL 1 DAY)")
		args = append(args, q.To+" 00:00:00")
	}
	if q.Q != "" {
		columns := []string{"booking_no", "temple_code", "temple_name", "master_code", "master_name", "service_code", "service_name"}
		if consultation {
			columns = []string{"order_no", "temple_code", "temple_name", "master_code", "master_name"}
		}
		pattern := "%" + strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(q.Q) + "%"
		matches := make([]string, 0, len(columns))
		for _, column := range columns {
			matches = append(matches, column+" LIKE ? ESCAPE '!'")
			args = append(args, pattern)
		}
		parts = append(parts, "("+strings.Join(matches, " OR ")+")")
	}
	return strings.Join(parts, " AND "), args
}

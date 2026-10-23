package logic

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/askxuan/booking-service/internal/model"
	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type ProgressRecord struct {
	ID        string        `db:"id" json:"id"`
	Kind      string        `db:"kind" json:"kind"`
	Content   string        `db:"content" json:"content"`
	Actor     string        `db:"operator_id" json:"operatorId"`
	ActorType string        `db:"operator_type" json:"operatorType"`
	CreatedAt string        `db:"created_at" json:"createdAt"`
	Files     []ReceiptFile `db:"-" json:"files"`
}
type ProgressRequest struct {
	ID      string   `json:"id"`
	Content string   `json:"content"`
	FileIds []string `json:"fileIds,optional"`
}

func ProgressRecords(ctx context.Context, s *svc.ServiceContext, id string) ([]ProgressRecord, error) {
	rows := []ProgressRecord{}
	err := s.DB.QueryRowsPartialCtx(ctx, &rows, "SELECT id,kind,content,operator_id,operator_type,DATE_FORMAT(create_time,'%Y-%m-%d %H:%i:%s') created_at FROM booking_progress WHERE booking_no=? ORDER BY sequence", id)
	if err != nil {
		return nil, err
	}
	for i := range rows {
		rows[i].Files = []ReceiptFile{}
		err = s.DB.QueryRowsCtx(ctx, &rows[i].Files, "SELECT "+receiptFileColumns+" FROM booking_receipt_file WHERE booking_no=? AND id IN (SELECT file_id FROM booking_progress_file WHERE progress_id=?) ORDER BY id", id, rows[i].ID)
		if err != nil {
			return nil, err
		}
	}
	return rows, nil
}
func PublishProgress(ctx context.Context, s *svc.ServiceContext, id, kind string, req ProgressRequest) (map[string]any, error) {
	_, actor, actorType, err := BookingAccess(ctx, s, id, kind == "update")
	if err != nil {
		return nil, err
	}
	if kind != "wish" && kind != "update" {
		return nil, common.ErrParamInvalid
	}
	if kind == "wish" && actorType != model.OperatorTypeUser {
		return nil, common.ErrForbidden
	}
	if _, err = uuid.Parse(req.ID); err != nil {
		return nil, common.ErrParamInvalid
	}
	req.Content = strings.TrimSpace(req.Content)
	max := 2000
	if kind == "wish" {
		max = 300
	}
	if len([]rune(req.Content)) < 2 || len([]rune(req.Content)) > max || len(req.FileIds) > 8 || (kind == "wish" && len(req.FileIds) > 0) {
		return nil, common.ErrParamInvalid
	}
	err = s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		status, e := lockBooking(ctx, tx, id)
		if e != nil {
			return e
		}
		// A retry of the same committed operation returns success without a second event.
		var count int
		if e = tx.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM booking_progress WHERE id=? AND booking_no=? AND operator_id=? AND operator_type=? AND kind=?", req.ID, id, actor, actorType, kind); e != nil {
			return e
		}
		if count > 0 {
			return nil
		}
		if kind == "update" && status != model.StatusInProgress {
			return common.ErrBookingStatusInvalid
		}
		if kind == "wish" && status != model.StatusPending && status != model.StatusConfirmed {
			return common.NewBizError(40034, "心愿可在服务开始前填写，开始后保留原记录")
		}
		if e = tx.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM booking_progress WHERE booking_no=?", id); e != nil {
			return e
		}
		if count >= 100 {
			return common.NewBizError(42934, "本次服务记录已达上限")
		}
		if _, e = tx.ExecCtx(ctx, "INSERT INTO booking_progress(id,booking_no,kind,content,operator_id,operator_type) VALUES(?,?,?,?,?,?)", req.ID, id, kind, req.Content, actor, actorType); e != nil {
			return e
		}
		seen := map[string]bool{}
		for _, fid := range req.FileIds {
			if seen[fid] {
				return common.ErrParamInvalid
			}
			seen[fid] = true
			var f ReceiptFile
			if e = tx.QueryRowCtx(ctx, &f, "SELECT "+receiptFileColumns+" FROM booking_receipt_file WHERE id=? FOR UPDATE", fid); e != nil {
				return common.ErrParamInvalid
			}
			if f.BookingId != id || f.Owner != actor || f.ReceiptId != "" {
				return common.ErrForbidden
			}
			if e = tx.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM booking_progress_file WHERE file_id=?", fid); e != nil {
				return e
			}
			if count > 0 {
				return common.NewBizError(40035, "该文件已用于阶段记录，请重新上传")
			}
			if _, e = tx.ExecCtx(ctx, "INSERT INTO booking_progress_file(progress_id,file_id) VALUES(?,?)", req.ID, fid); e != nil {
				return e
			}
		}
		if kind == "update" {
			_, e = tx.ExecCtx(ctx, `INSERT INTO event_outbox(event_key,aggregate_type,aggregate_id,event_type,exchange_name,routing_key,payload,status,retry_count,next_retry_at,created_at,updated_at)
    SELECT CONCAT('booking:progress:',?), 'booking',booking_no,'booking.progress','booking.events','',JSON_OBJECT('bookingId',booking_no,'userId',CAST(user_id AS CHAR),'action','progress','time',DATE_FORMAT(NOW(),'%Y-%m-%d %H:%i:%s')), 'pending',0,NOW(),NOW(),NOW() FROM booking WHERE booking_no=?`, req.ID, id)
		}
		return e
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"id": req.ID}, nil
}

type JourneyRow struct {
	Preview      []ReceiptFile `db:"-" json:"preview"`
	ID           string        `db:"id" json:"id"`
	Temple       string        `db:"temple_name" json:"templeName"`
	Master       string        `db:"master_name" json:"masterName"`
	MasterCode   string        `db:"master_code" json:"masterId"`
	Service      string        `db:"service_name" json:"serviceName"`
	Date         string        `db:"date" json:"bookingDate"`
	Time         string        `db:"time_slot" json:"timeSlot"`
	Status       string        `db:"status" json:"status"`
	Latest       string        `db:"latest" json:"latest"`
	Updated      string        `db:"updated" json:"updatedAt"`
	ReceiptCount int           `db:"receipt_count" json:"receiptCount"`
	Revision     int           `db:"revision" json:"needsRevision"`
}

func JourneyList(ctx context.Context, s *svc.ServiceContext, page int, filter, search, from, to string) (map[string]any, error) {
	if page < 1 || page > 10000 || len([]rune(search)) > 60 {
		return nil, common.ErrParamInvalid
	}
	uid := middleware.UserIDFromCtx(ctx)
	if uid <= 0 {
		return nil, common.ErrUnauthorized
	}
	scope := ""
	args := []any{}
	for _, role := range middleware.RolesFromCtx(ctx) {
		switch role {
		case "customer":
			scope = "b.user_id=?"
			args = []any{strconv.FormatInt(uid, 10)}
		case "temple_admin":
			if code := middleware.TempleCodeFromCtx(ctx); code != "" {
				scope = "b.temple_code=?"
				args = []any{code}
			}
		case "master":
			if mid := middleware.MasterIDFromCtx(ctx); mid > 0 {
				m, e := s.MasterClient.GetByID(ctx, mid)
				if e != nil {
					return nil, common.ErrDependencyUnavailable
				}
				if m.Code != "" {
					scope = "b.master_code=?"
					args = []any{m.Code}
				}
			}
		}
		if scope != "" {
			break
		}
	}
	if scope == "" {
		return nil, common.ErrForbidden
	}
	base := " FROM booking b WHERE " + scope + " AND b.payment_status='success'"
	var counts struct {
		Active    int `db:"active" json:"active"`
		Pending   int `db:"pending" json:"pending"`
		Confirmed int `db:"confirmed" json:"confirmed"`
		Executing int `db:"executing" json:"executing"`
		Receipt   int `db:"receipt" json:"receipt"`
		Complete  int `db:"complete" json:"complete"`
		Revision  int `db:"revision" json:"revision"`
	}
	revision := "(b.status='in_progress' AND (SELECT from_status FROM booking_status_log WHERE booking_id=b.booking_no ORDER BY id DESC LIMIT 1)='pending_receipt')"
	err := s.DB.QueryRowCtx(ctx, &counts, "SELECT COALESCE(SUM(b.status IN ('pending','confirmed','in_progress')),0) active,COALESCE(SUM(b.status='pending'),0) pending,COALESCE(SUM(b.status='confirmed'),0) confirmed,COALESCE(SUM(b.status='in_progress'),0) executing,COALESCE(SUM(b.status='pending_receipt'),0) receipt,COALESCE(SUM(b.status IN ('completed','reviewed')),0) complete,COALESCE(SUM("+revision+"),0) revision"+base, args...)
	if err != nil {
		return nil, err
	}
	switch filter {
	case "all":
	case "active":
		base += " AND b.status IN ('pending','confirmed','in_progress')"
	case "receipt":
		base += " AND b.status='pending_receipt'"
	case "archive":
		base += " AND EXISTS (SELECT 1 FROM booking_receipt r WHERE r.booking_no=b.booking_no)"
	case "complete":
		base += " AND b.status IN ('completed','reviewed')"
	case "pending", "confirmed", "in_progress":
		base += " AND b.status=?"
		args = append(args, filter)
	case "revision":
		base += " AND " + revision
	default:
		return nil, common.ErrParamInvalid
	}
	if search != "" {
		base += " AND (b.temple_name LIKE ? OR b.master_name LIKE ? OR b.service_name LIKE ?)"
		term := "%" + search + "%"
		args = append(args, term, term, term)
	}
	for _, d := range []string{from, to} {
		if d != "" && !validJourneyDate(d) {
			return nil, common.ErrParamInvalid
		}
	}
	if from != "" && to != "" && from > to {
		return nil, common.ErrParamInvalid
	}
	if from != "" {
		base += " AND b.booking_date>=?"
		args = append(args, from)
	}
	if to != "" {
		base += " AND b.booking_date<=?"
		args = append(args, to)
	}
	var total int
	if err = s.DB.QueryRowCtx(ctx, &total, "SELECT COUNT(*)"+base, args...); err != nil {
		return nil, err
	}
	rows := []JourneyRow{}
	latest := "COALESCE((SELECT content FROM booking_progress WHERE booking_no=b.booking_no AND kind='update' AND create_time>=b.update_time ORDER BY sequence DESC LIMIT 1),'')"
	if filter == "archive" {
		latest = "COALESCE((SELECT summary FROM booking_receipt WHERE booking_no=b.booking_no ORDER BY sequence DESC LIMIT 1),'')"
	}
	query := `SELECT b.booking_no id,b.temple_name,b.master_name,b.master_code,b.service_name,DATE_FORMAT(b.booking_date,'%Y-%m-%d') date,b.time_slot,b.status,
 ` + latest + ` latest,
 DATE_FORMAT(GREATEST(b.update_time,COALESCE((SELECT MAX(create_time) FROM booking_progress WHERE booking_no=b.booking_no),b.update_time)),'%Y-%m-%d %H:%i:%s') updated,
 (SELECT COUNT(*) FROM booking_receipt WHERE booking_no=b.booking_no) receipt_count,COALESCE(` + revision + `,0) revision` + base + ` ORDER BY CASE WHEN b.status='pending_receipt' THEN 0 WHEN ` + revision + ` THEN 1 WHEN b.status='in_progress' THEN 2 WHEN b.status='confirmed' THEN 3 WHEN b.status='pending' THEN 4 ELSE 5 END,updated DESC,b.id DESC LIMIT 20 OFFSET ?`
	args = append(args, (page-1)*20)
	if err = s.DB.QueryRowsPartialCtx(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	// Metadata is loaded only for the bounded archive page. Private bytes still require BookingAccess.
	for i := range rows {
		rows[i].Preview = []ReceiptFile{}
	}
	if filter == "archive" && len(rows) > 0 {
		ids := []any{}
		marks := []string{}
		byID := map[string]int{}
		for i, row := range rows {
			ids = append(ids, row.ID)
			marks = append(marks, "?")
			byID[row.ID] = i
		}
		files := []ReceiptFile{}
		if err = s.DB.QueryRowsCtx(ctx, &files, "SELECT "+receiptFileColumns+" FROM booking_receipt_file f WHERE booking_no IN ("+strings.Join(marks, ",")+") AND receipt_id=(SELECT id FROM booking_receipt r WHERE r.booking_no=f.booking_no ORDER BY sequence DESC LIMIT 1) ORDER BY id", ids...); err != nil {
			return nil, err
		}
		for _, f := range files {
			i := byID[f.BookingId]
			if len(rows[i].Preview) < 3 {
				rows[i].Preview = append(rows[i].Preview, f)
			}
		}
	}
	return map[string]any{"list": rows, "total": total, "page": page, "counts": counts}, nil
}

func validJourneyDate(value string) bool {
	_, err := time.Parse("2006-01-02", value)
	return err == nil
}

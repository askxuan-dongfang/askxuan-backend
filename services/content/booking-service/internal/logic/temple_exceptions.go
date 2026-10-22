package logic

import (
	"context"
	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"regexp"
	"strings"
	"unicode/utf8"
)

type TempleException struct {
	ID         string `db:"id" json:"id"`
	TempleCode string `db:"temple_code" json:"templeCode"`
	SourceKind string `db:"source_kind" json:"sourceKind"`
	SourceID   string `db:"source_id" json:"sourceId"`
	Title      string `db:"title" json:"title"`
	Reason     string `db:"reason" json:"reason"`
	Status     string `db:"status" json:"status"`
	Followup   string `db:"followup" json:"followup"`
	UpdatedAt  string `db:"updated_at" json:"updatedAt"`
}
type ExceptionNote struct {
	ID           int64  `db:"id" json:"id"`
	OperatorID   int64  `db:"operator_id" json:"operatorId"`
	OperatorRole string `db:"operator_role" json:"operatorRole"`
	Action       string `db:"action" json:"action"`
	Content      string `db:"content" json:"content"`
	CreatedAt    string `db:"created_at" json:"createdAt"`
}

// Conditions always derive from the canonical business state. Notes never complete/refund an order.
const exceptionSources = `SELECT CONCAT('booking:',b.booking_no) id,b.temple_code,'booking' source_kind,b.booking_no source_id,b.service_name title,b.status,
CASE WHEN b.payment_status='refunding' THEN 'refund_pending'
 WHEN b.status='in_progress' AND (SELECT from_status FROM booking_status_log WHERE booking_id=b.booking_no ORDER BY id DESC LIMIT 1)='pending_receipt' THEN 'receipt_revision'
 WHEN b.status IN ('pending','confirmed','in_progress') AND b.booking_date<CURDATE() THEN 'overdue'
 WHEN b.status IN ('pending','confirmed','in_progress') AND b.booking_date>=CURDATE() AND EXISTS (SELECT 1 FROM askxuan_master.master_schedule ms WHERE ms.master_code=b.master_code AND ms.schedule_date=b.booking_date AND (ms.status<>'available' OR NOT EXISTS(SELECT 1 FROM JSON_TABLE(IF(JSON_VALID(ms.time_slots),ms.time_slots,'[]'),'$[*]' COLUMNS(slot VARCHAR(32) PATH '$')) ts WHERE SUBSTRING_INDEX(ts.slot,'-',1)<=SUBSTRING_INDEX(b.time_slot,'-',1) AND SUBSTRING_INDEX(ts.slot,'-',-1)>=SUBSTRING_INDEX(b.time_slot,'-',-1)))) THEN 'schedule_conflict'
 ELSE '' END reason
 FROM booking b WHERE b.temple_code<>'' AND (b.payment_status IN ('success','refunding'))
UNION ALL SELECT CONCAT('blessing:',t.id),t.temple_code,'blessing',CAST(t.id AS CHAR),CONCAT('DIY ',t.task_no),t.status,
 CASE WHEN t.status='rejected' THEN 'blessing_rejected' WHEN t.status='dispatched' THEN 'blessing_unassigned' ELSE '' END
 FROM askxuan_diy.blessing_task t WHERE t.temple_code<>''`

func exceptionScope(ctx context.Context, requested string, platform bool) (string, error) {
	c := middleware.AdminClaimsFromCtx(ctx)
	if c == nil || c.UserType != "admin" {
		return "", common.ErrForbidden
	}
	if platform {
		if !c.HasRole("platform_super") {
			return "", common.ErrForbidden
		}
		return requested, nil
	}
	if !(c.HasRole("temple_admin") || c.HasRole("temple_reception") || c.HasRole("temple_operations")) || c.TempleCode == "" || (requested != "" && requested != c.TempleCode) {
		return "", common.ErrForbidden
	}
	return c.TempleCode, nil
}
func TempleExceptions(ctx context.Context, s *svc.ServiceContext, requested string, platform bool, page int) (map[string]any, error) {
	code, e := exceptionScope(ctx, requested, platform)
	if e != nil {
		return nil, e
	}
	if page < 1 || page > 10000 {
		return nil, common.ErrParam
	}
	where := " WHERE e.reason<>''"
	args := []any{}
	if code != "" {
		where += " AND e.temple_code=?"
		args = append(args, code)
	}
	base := " FROM (" + exceptionSources + ") e" + where
	var total int64
	if e = s.DB.QueryRowCtx(ctx, &total, "SELECT COUNT(*)"+base, args...); e != nil {
		return nil, e
	}
	rows := []TempleException{}
	query := `SELECT e.*,COALESCE((SELECT action FROM temple_exception_note n WHERE n.temple_code=e.temple_code AND n.source_kind=e.source_kind AND n.source_id=e.source_id ORDER BY n.id DESC LIMIT 1),'unhandled') followup,COALESCE((SELECT DATE_FORMAT(create_time,'%Y-%m-%d %H:%i:%s') FROM temple_exception_note n WHERE n.temple_code=e.temple_code AND n.source_kind=e.source_kind AND n.source_id=e.source_id ORDER BY n.id DESC LIMIT 1),'') updated_at` + base + ` ORDER BY e.source_kind,e.source_id DESC LIMIT 20 OFFSET ?`
	args = append(args, (page-1)*20)
	e = s.DB.QueryRowsCtx(ctx, &rows, query, args...)
	return map[string]any{"list": rows, "total": total, "page": page, "size": 20}, e
}

var exceptionID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func exceptionSource(ctx context.Context, tx sqlx.Session, id string) (kind, source, temple string, err error) {
	p := strings.Split(id, ":")
	if len(p) != 2 || !exceptionID.MatchString(p[1]) {
		err = common.ErrParam
		return
	}
	kind, source = p[0], p[1]
	switch kind {
	case "booking":
		err = tx.QueryRowCtx(ctx, &temple, `SELECT temple_code FROM booking WHERE booking_no=?`, source)
	case "blessing":
		err = tx.QueryRowCtx(ctx, &temple, `SELECT temple_code FROM askxuan_diy.blessing_task WHERE id=?`, source)
	default:
		err = common.ErrParam
	}
	if err == sqlx.ErrNotFound {
		err = common.ErrForbidden
	}
	return
}
func ExceptionNotes(ctx context.Context, s *svc.ServiceContext, id string, platform bool) ([]ExceptionNote, error) {
	code, e := exceptionScope(ctx, "", platform)
	if e != nil {
		return nil, e
	}
	kind, source, temple, e := exceptionSource(ctx, s.DB, id)
	if e != nil {
		return nil, e
	}
	if temple == "" || (code != "" && temple != code) {
		return nil, common.ErrForbidden
	}
	rows := []ExceptionNote{}
	e = s.DB.QueryRowsCtx(ctx, &rows, `SELECT id,operator_id,operator_role,action,content,DATE_FORMAT(create_time,'%Y-%m-%d %H:%i:%s') created_at FROM temple_exception_note WHERE temple_code=? AND source_kind=? AND source_id=? ORDER BY id DESC LIMIT 100`, temple, kind, source)
	return rows, e
}
func AddExceptionNote(ctx context.Context, s *svc.ServiceContext, id, requestID, action, content string) error {
	code, e := exceptionScope(ctx, "", false)
	if e != nil {
		return e
	}
	content = strings.TrimSpace(content)
	if len(requestID) < 16 || len(requestID) > 64 || !exceptionID.MatchString(requestID) || content == "" || utf8.RuneCountInString(content) > 1000 || (action != "following" && action != "escalated") {
		return common.ErrParam
	}
	c := middleware.AdminClaimsFromCtx(ctx)
	return s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		kind, source, temple, e := exceptionSource(ctx, tx, id)
		if e != nil {
			return e
		}
		if temple != code {
			return common.ErrForbidden
		}
		var existing struct {
			ID      string `db:"source_id"`
			Kind    string `db:"source_kind"`
			Action  string `db:"action"`
			Content string `db:"content"`
		}
		e = tx.QueryRowCtx(ctx, &existing, `SELECT source_id,source_kind,action,content FROM temple_exception_note WHERE temple_code=? AND operator_id=? AND request_id=?`, code, c.UserId, requestID)
		if e == nil {
			if existing.ID == source && existing.Kind == kind && existing.Action == action && existing.Content == content {
				return nil
			}
			return common.ErrParam
		}
		if e != sqlx.ErrNotFound {
			return e
		}
		_, e = tx.ExecCtx(ctx, `INSERT INTO temple_exception_note(temple_code,source_kind,source_id,operator_id,operator_role,request_id,action,content) VALUES(?,?,?,?,?,?,?,?)`, code, kind, source, c.UserId, strings.Join(c.Roles, ","), requestID, action, content)
		return e
	})
}

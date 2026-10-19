package growth

import (
	"context"
	"errors"
	"fmt"
	"github.com/askxuan/common"
	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"strings"
	"time"
)

type Content struct {
	ID       int64  `db:"id" json:"id"`
	Title    string `db:"title" json:"title"`
	Kind     string `db:"kind" json:"kind"`
	Body     string `db:"body" json:"body"`
	MediaURL string `db:"media_url" json:"mediaUrl"`
	Duration int64  `db:"duration_seconds" json:"durationSeconds"`
	Enabled  bool   `db:"enabled" json:"enabled"`
	Revision int64  `db:"revision" json:"revision"`
}

const contentFields = "id,title,kind,body,media_url,duration_seconds,enabled,revision"

func (s *Store) Contents(ctx context.Context, admin bool) ([]Content, error) {
	v := []Content{}
	q := "SELECT " + contentFields + " FROM growth_content"
	if !admin {
		q += " WHERE enabled=1"
	}
	e := s.DB.QueryRowsCtx(ctx, &v, q+" ORDER BY id DESC LIMIT 100")
	return v, e
}

type Learning struct {
	ID        string `db:"session_id" json:"sessionId"`
	User      string `db:"user_id" json:"-"`
	ContentID int64  `db:"content_id" json:"contentId"`
	Revision  int64  `db:"content_revision" json:"revision"`
	Elapsed   int64  `db:"elapsed_seconds" json:"elapsedSeconds"`
	Last      int64  `db:"last_seen" json:"-"`
	Expires   int64  `db:"expires_at" json:"expiresAt"`
	Completed bool   `db:"completed" json:"completed"`
}

func (s *Store) StartLearning(ctx context.Context, user string, id int64) (v Learning, e error) {
	var c Content
	e = s.DB.QueryRowCtx(ctx, &c, "SELECT "+contentFields+" FROM growth_content WHERE id=? AND enabled=1", id)
	if e != nil {
		return v, e
	}
	now := s.now().Unix()
	v = Learning{ID: strings.ReplaceAll(uuid.NewString(), "-", ""), User: user, ContentID: id, Revision: c.Revision, Last: now, Expires: now + 7200}
	_, e = s.DB.ExecCtx(ctx, "INSERT INTO growth_learning_session(session_id,user_id,content_id,content_revision,last_seen,expires_at) VALUES(?,?,?,?,?,?)", v.ID, user, id, c.Revision, now, v.Expires)
	return
}
func (s *Store) LearningHeartbeat(ctx context.Context, user, id string, visible, reachedEnd bool) (v Learning, e error) {
	if len(id) != 32 {
		return v, common.ErrParam
	}
	e = s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		a, err := lockAccount(ctx, tx, user, s.now().Unix())
		if err != nil {
			return err
		}
		err = tx.QueryRowCtx(ctx, &v, "SELECT session_id,user_id,content_id,content_revision,elapsed_seconds,last_seen,expires_at,completed FROM growth_learning_session WHERE session_id=? AND user_id=? FOR UPDATE", id, user)
		if err != nil {
			return err
		}
		if v.Completed {
			return nil
		}
		now := s.now().Unix()
		if now > v.Expires {
			return ErrChanged
		}
		var c Content
		if err = tx.QueryRowCtx(ctx, &c, "SELECT "+contentFields+" FROM growth_content WHERE id=? AND enabled=1", v.ContentID); err != nil {
			return err
		}
		if c.Revision != v.Revision {
			return ErrChanged
		}
		delta := now - v.Last
		if delta < 0 {
			delta = 0
		}
		if delta > 15 {
			delta = 0
		}
		if !visible {
			delta = 0
		}
		v.Elapsed += delta
		v.Last = now
		needed := int64(60)
		if c.Kind == "media" {
			needed = (c.Duration*8 + 9) / 10
		}
		if reachedEnd && visible && needed > 0 && v.Elapsed >= needed {
			_, err = s.awardTask(ctx, tx, &a, "learning", fmt.Sprint(c.ID), 3, 1, "day")
			if err != nil && !errors.Is(err, ErrLimit) {
				return err
			}
			v.Completed = true
		}
		_, err = tx.ExecCtx(ctx, "UPDATE growth_learning_session SET elapsed_seconds=?,last_seen=?,completed=? WHERE session_id=?", v.Elapsed, now, v.Completed, id)
		return err
	})
	return
}

type Note struct {
	Day     string `db:"business_day" json:"day"`
	Action  string `db:"action" json:"action"`
	Body    string `db:"body" json:"body"`
	Updated int64  `db:"updated_at" json:"updatedAt"`
}

func (s *Store) Notes(ctx context.Context, user string) (v []Note, e error) {
	v = []Note{}
	e = s.DB.QueryRowsCtx(ctx, &v, "SELECT business_day,action,body,updated_at FROM growth_note WHERE user_id=? AND deleted=0 ORDER BY business_day DESC LIMIT 100", user)
	return
}
func (s *Store) DeleteNote(ctx context.Context, user, day string) error {
	if _, e := time.Parse("2006-01-02", day); e != nil {
		return common.ErrParam
	}
	_, e := s.DB.ExecCtx(ctx, "UPDATE growth_note SET action='',body='',deleted=1,updated_at=? WHERE user_id=? AND business_day=?", s.now().Unix(), user, day)
	return e
}

type Activity struct {
	ID          int64  `db:"id" json:"id"`
	Title       string `db:"title" json:"title"`
	Description string `db:"description" json:"description"`
	Starts      int64  `db:"starts_at" json:"startsAt"`
	Ends        int64  `db:"ends_at" json:"endsAt"`
	Enabled     bool   `db:"enabled" json:"enabled"`
	Registered  int64  `db:"registered_at" json:"registeredAt"`
	Verified    int64  `db:"verified_at" json:"verifiedAt"`
}

func (s *Store) Activities(ctx context.Context, user string, admin bool) (v []Activity, e error) {
	v = []Activity{}
	q := "SELECT a.id,a.title,a.description,a.starts_at,a.ends_at,a.enabled,COALESCE(r.registered_at,0) registered_at,COALESCE(r.verified_at,0) verified_at FROM growth_activity a LEFT JOIN growth_attendance r ON r.activity_id=a.id AND r.user_id=?"
	if !admin {
		q += " WHERE a.enabled=1"
	}
	e = s.DB.QueryRowsCtx(ctx, &v, q+" ORDER BY a.id DESC LIMIT 100", user)
	return
}
func (s *Store) RegisterActivity(ctx context.Context, user string, id int64) error {
	now := s.now().Unix()
	result, e := s.DB.ExecCtx(ctx, "INSERT INTO growth_attendance(activity_id,user_id,registered_at) SELECT id,?,? FROM growth_activity WHERE id=? AND enabled=1 AND ends_at>=? ON DUPLICATE KEY UPDATE registered_at=registered_at", user, now, id, now)
	if e != nil {
		return e
	}
	n, e := result.RowsAffected()
	if n == 0 {
		var count int64
		e = s.DB.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM growth_attendance WHERE activity_id=? AND user_id=?", id, user)
		if e == nil && count == 0 {
			return ErrChanged
		}
	}
	return e
}
func (s *Store) VerifyAttendance(ctx context.Context, operator, user, evidence string, id int64) error {
	if user == "" || len(user) > 64 || strings.TrimSpace(evidence) == "" || len([]rune(evidence)) > 255 {
		return common.ErrParam
	}
	return s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		a, e := lockAccount(ctx, tx, user, s.now().Unix())
		if e != nil {
			return e
		}
		var verified int64
		e = tx.QueryRowCtx(ctx, &verified, "SELECT verified_at FROM growth_attendance WHERE activity_id=? AND user_id=? FOR UPDATE", id, user)
		if e != nil {
			return e
		}
		if verified > 0 {
			return nil
		}
		var n int64
		e = tx.QueryRowCtx(ctx, &n, "SELECT COUNT(*) FROM growth_activity WHERE id=? AND enabled=1 AND starts_at<=?", id, s.now().Unix())
		if e != nil {
			return e
		}
		if n != 1 {
			return ErrChanged
		}
		_, e = s.awardTask(ctx, tx, &a, "activity", fmt.Sprint(id), 20, 2, "month")
		if e != nil && !errors.Is(e, ErrLimit) {
			return e
		}
		_, e = tx.ExecCtx(ctx, "UPDATE growth_attendance SET verified_at=?,verified_by=?,evidence=? WHERE activity_id=? AND user_id=?", s.now().Unix(), operator, evidence, id, user)
		if e != nil {
			return e
		}
		return audit(ctx, tx, operator, "attendance", fmt.Sprint(id), map[string]any{"userId": user, "evidence": evidence}, s.now().Unix())
	})
}

type Summary struct {
	Period   string          `json:"period"`
	Added    int64           `db:"added" json:"added"`
	Reversed int64           `db:"reversed" json:"reversed"`
	Entries  int64           `db:"entries" json:"entries"`
	BySource []SourceSummary `json:"bySource"`
}
type SourceSummary struct {
	Kind   string `db:"kind" json:"kind"`
	Amount int64  `db:"amount" json:"amount"`
	Count  int64  `db:"count" json:"count"`
}

func (s *Store) Summary(ctx context.Context, user, period string, yearly bool) (v Summary, e error) {
	a, e := s.Overview(ctx, user)
	if e != nil {
		return v, e
	}
	min := 3
	layout := "2006-01"
	if yearly {
		min = 5
		layout = "2006"
	}
	if a.Level.Code < min {
		return v, common.ErrForbidden
	}
	start, e := time.ParseInLocation(layout, period, Shanghai)
	if e != nil {
		return v, common.ErrParam
	}
	end := start.AddDate(0, 1, 0)
	if yearly {
		end = start.AddDate(1, 0, 0)
	}
	if start.After(s.now()) {
		return v, common.ErrParam
	}
	v.Period = period
	v.BySource = []SourceSummary{}
	e = s.DB.QueryRowPartialCtx(ctx, &v, "SELECT COALESCE(SUM(GREATEST(delta,0)),0) added,COALESCE(SUM(LEAST(delta,0)),0) reversed,COUNT(*) entries FROM growth_ledger WHERE user_id=? AND created_at>=? AND created_at<?", user, start.Unix(), end.Unix())
	if e != nil {
		return v, e
	}
	e = s.DB.QueryRowsCtx(ctx, &v.BySource, "SELECT kind,SUM(delta) amount,COUNT(*) count FROM growth_ledger WHERE user_id=? AND created_at>=? AND created_at<? GROUP BY kind ORDER BY kind", user, start.Unix(), end.Unix())
	return
}

type Support struct {
	ID       int64  `db:"id" json:"id"`
	User     string `db:"user_id" json:"userId"`
	Category string `db:"category" json:"category"`
	Body     string `db:"body" json:"body"`
	Level    int    `db:"level" json:"level"`
	Urgency  int    `db:"urgency" json:"urgency"`
	Status   string `db:"status" json:"status"`
	Reply    string `db:"reply" json:"reply"`
	Created  int64  `db:"created_at" json:"createdAt"`
}

func (s *Store) Support(ctx context.Context, user string, admin bool) (v []Support, e error) {
	v = []Support{}
	q := "SELECT id,user_id,category,body,level,urgency,status,reply,created_at FROM growth_support"
	args := []any{}
	if !admin {
		q += " WHERE user_id=?"
		args = append(args, user)
	}
	q += " ORDER BY (status='queued') DESC,urgency DESC,(level>=4) DESC,created_at ASC LIMIT 100"
	e = s.DB.QueryRowsCtx(ctx, &v, q, args...)
	return
}
func (s *Store) RequestSupport(ctx context.Context, user, request, category, body string) error {
	if len(request) < 8 || len(request) > 64 || len([]rune(body)) < 2 || len([]rune(body)) > 2000 {
		return common.ErrParam
	}
	if category != "growth" && category != "order" && category != "refund" {
		return common.ErrParam
	}
	return s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		a, e := lockAccount(ctx, tx, user, s.now().Unix())
		if e != nil {
			return e
		}
		var existing int
		if e = tx.QueryRowCtx(ctx, &existing, "SELECT COUNT(*) FROM growth_support WHERE user_id=? AND request_id=?", user, request); e != nil {
			return e
		}
		if existing > 0 {
			return nil
		}
		var count int
		e = tx.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM growth_support WHERE user_id=? AND status='queued'", user)
		if e != nil {
			return e
		}
		if count >= 10 {
			return ErrLimit
		}
		_, e = tx.ExecCtx(ctx, "INSERT INTO growth_support(user_id,request_id,category,body,level,urgency,reply,created_at,updated_at) VALUES(?,?,?,?,?,0,'',?,?) ON DUPLICATE KEY UPDATE request_id=request_id", user, request, category, body, Tier(a.Total).Code, s.now().Unix(), s.now().Unix())
		return e
	})
}

package growth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"strings"
	"time"
)

var ErrUnavailable = common.NewBizError(40940, "成长服务暂未开放")
var ErrChanged = common.NewBizError(40941, "成长规则或优惠已变化，请刷新后重新确认")
var ErrLimit = common.NewBizError(40942, "该任务已完成或本期额度已用完")

type Store struct {
	DB  sqlx.SqlConn
	Now func() time.Time
}

func (s *Store) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

type Settings struct {
	Enabled   bool   `db:"enabled" json:"enabled"`
	Discounts bool   `db:"discounts_enabled" json:"discountsEnabled"`
	Revision  int64  `db:"revision" json:"revision"`
	Version   string `db:"rule_version" json:"ruleVersion"`
	Started   int64  `db:"started_at" json:"startedAt"`
}

func (s *Store) Settings(ctx context.Context) (v Settings, e error) {
	e = s.DB.QueryRowCtx(ctx, &v, "SELECT enabled,discounts_enabled,revision,rule_version,started_at FROM growth_settings WHERE id=1")
	return
}

type Account struct {
	User         string `db:"user_id" json:"-"`
	Total        int64  `db:"total" json:"total"`
	Revision     int64  `db:"revision" json:"revision"`
	BadgeVisible bool   `db:"badge_visible" json:"badgeVisible"`
	Updated      int64  `db:"updated_at" json:"updatedAt"`
}

func lockAccount(ctx context.Context, tx sqlx.Session, user string, now int64) (a Account, e error) {
	if user == "" || len(user) > 64 {
		return a, common.ErrParam
	}
	_, e = tx.ExecCtx(ctx, "INSERT INTO growth_account(user_id,updated_at) VALUES(?,?) ON DUPLICATE KEY UPDATE user_id=user_id", user, now)
	if e == nil {
		e = tx.QueryRowCtx(ctx, &a, "SELECT user_id,total,revision,badge_visible,updated_at FROM growth_account WHERE user_id=? FOR UPDATE", user)
	}
	return
}

type Event struct {
	ID        int64  `db:"id" json:"id"`
	Key       string `db:"event_key" json:"-"`
	User      string `db:"user_id" json:"-"`
	Kind      string `db:"kind" json:"kind"`
	Reference string `db:"reference_no" json:"referenceNo"`
	Original  int64  `db:"original_award" json:"originalAward"`
	Current   int64  `db:"current_award" json:"amount"`
	Applied   int64  `db:"applied" json:"applied"`
	State     string `db:"state" json:"state"`
	Reason    string `db:"reason" json:"reason"`
	Eligible  int64  `db:"eligible_at" json:"eligibleAt"`
	Day       string `db:"business_day" json:"businessDay"`
	Month     string `db:"business_month" json:"businessMonth"`
	Version   string `db:"rule_version" json:"ruleVersion"`
	Created   int64  `db:"created_at" json:"createdAt"`
	Updated   int64  `db:"updated_at" json:"updatedAt"`
}

const eventFields = "id,event_key,user_id,kind,reference_no,original_award,current_award,applied,state,reason,eligible_at,business_day,business_month,rule_version,created_at,updated_at"

type Entry struct {
	ID      int64  `db:"id" json:"id"`
	Kind    string `db:"kind" json:"kind"`
	Ref     string `db:"reference_no" json:"referenceNo"`
	Delta   int64  `db:"delta" json:"delta"`
	Total   int64  `db:"total_after" json:"totalAfter"`
	Created int64  `db:"created_at" json:"createdAt"`
}
type Task struct {
	Kind      string `json:"kind"`
	Award     int64  `json:"award"`
	Limit     int64  `json:"limit"`
	Used      int64  `json:"used"`
	Available bool   `json:"available"`
	Period    string `json:"period"`
}
type Overview struct {
	Account
	Level            Level    `json:"level"`
	Levels           []Level  `json:"levels"`
	Next             int64    `json:"nextThreshold"`
	Remaining        int64    `json:"remaining"`
	Pending          int64    `json:"pending"`
	MonthlyEarned    int64    `json:"monthlyEarned"`
	MonthlyLimit     int64    `json:"monthlyLimit"`
	Tasks            []Task   `json:"tasks"`
	Coupons          []Coupon `json:"coupons"`
	Version          string   `json:"ruleVersion"`
	DiscountsEnabled bool     `json:"discountsEnabled"`
}

func (s *Store) Overview(ctx context.Context, user string) (v Overview, e error) {
	cfg, e := s.Settings(ctx)
	if e != nil {
		return v, e
	}
	if !cfg.Enabled {
		return v, ErrUnavailable
	}
	e = s.DB.QueryRowCtx(ctx, &v.Account, "SELECT user_id,total,revision,badge_visible,updated_at FROM growth_account WHERE user_id=?", user)
	if errors.Is(e, sqlx.ErrNotFound) {
		v.Account = Account{User: user, BadgeVisible: true}
		e = nil
	}
	if e != nil {
		return v, e
	}
	v.Level = Tier(v.Total)
	v.Levels = Levels()
	v.Version = cfg.Version
	v.DiscountsEnabled = cfg.Discounts
	v.MonthlyLimit = MonthlyGrowthLimit
	v.Tasks = []Task{}
	v.Coupons = []Coupon{}
	if v.Level.Code < 5 {
		v.Next = v.Levels[v.Level.Code].Threshold
		v.Remaining = v.Next - v.Total
	}
	e = s.DB.QueryRowCtx(ctx, &v.Pending, "SELECT COALESCE(SUM(current_award-applied),0) FROM growth_event WHERE user_id=? AND state IN ('pending','paused')", user)
	if e != nil {
		return v, e
	}
	now := s.now()
	e = s.DB.QueryRowCtx(ctx, &v.MonthlyEarned, "SELECT COALESCE(SUM(original_award),0) FROM growth_event WHERE user_id=? AND business_month=? AND kind IN ('learning','note','activity','review')", user, BusinessMonth(now))
	if e != nil {
		return v, e
	}
	for _, t := range []Task{{Kind: "profile", Award: 20, Limit: 1, Period: "lifetime", Available: true}, {Kind: "learning", Award: 3, Limit: 1, Period: "day"}, {Kind: "note", Award: 2, Limit: 1, Period: "day", Available: true}, {Kind: "activity", Award: 20, Limit: 2, Period: "month"}, {Kind: "review", Award: 5, Limit: 4, Period: "month", Available: true}} {
		query := "SELECT COUNT(*) FROM growth_event WHERE user_id=? AND kind=?"
		args := []any{user, t.Kind}
		if t.Period == "day" {
			query += " AND business_day=?"
			args = append(args, BusinessDay(now))
		}
		if t.Period == "month" {
			query += " AND business_month=?"
			args = append(args, BusinessMonth(now))
		}
		if e = s.DB.QueryRowCtx(ctx, &t.Used, query, args...); e != nil {
			return v, e
		}
		var n int64
		if t.Kind == "learning" {
			e = s.DB.QueryRowCtx(ctx, &n, "SELECT COUNT(*) FROM growth_content WHERE enabled=1")
			t.Available = n > 0
		}
		if t.Kind == "activity" {
			e = s.DB.QueryRowCtx(ctx, &n, "SELECT COUNT(*) FROM growth_activity WHERE enabled=1 AND ends_at>=?", now.Unix())
			t.Available = n > 0
		}
		if e != nil {
			return v, e
		}
		v.Tasks = append(v.Tasks, t)
	}
	e = s.DB.QueryRowsCtx(ctx, &v.Coupons, "SELECT id,level,minimum_cents,value_cents,status,expires_at FROM growth_coupon WHERE user_id=? ORDER BY level", user)
	return
}
func (s *Store) Records(ctx context.Context, user string, page int) (map[string]any, error) {
	if page < 1 || page > 100000 {
		return nil, common.ErrParam
	}
	entries := []Entry{}
	events := []Event{}
	e := s.DB.QueryRowsCtx(ctx, &entries, "SELECT id,kind,reference_no,delta,total_after,created_at FROM growth_ledger WHERE user_id=? ORDER BY id DESC LIMIT 30 OFFSET ?", user, (page-1)*30)
	if e != nil {
		return nil, e
	}
	e = s.DB.QueryRowsCtx(ctx, &events, "SELECT "+eventFields+" FROM growth_event WHERE user_id=? AND state IN ('pending','paused') ORDER BY id DESC LIMIT 100", user)
	return map[string]any{"entries": entries, "pending": events, "page": page, "hasMore": len(entries) == 30}, e
}
func (s *Store) awardTask(ctx context.Context, tx sqlx.Session, a *Account, kind, ref string, amount, limit int64, period string) (Event, error) {
	return s.awardTaskAt(ctx, tx, a, kind, ref, amount, limit, period, s.now(), false)
}
func (s *Store) awardTaskAt(ctx context.Context, tx sqlx.Session, a *Account, kind, ref string, amount, limit int64, period string, now time.Time, recordLimit bool) (Event, error) {
	key := a.User + ":" + kind + ":" + ref
	var event Event
	e := tx.QueryRowCtx(ctx, &event, "SELECT "+eventFields+" FROM growth_event WHERE event_key=?", key)
	if e == nil {
		return event, nil
	}
	if !errors.Is(e, sqlx.ErrNotFound) {
		return event, e
	}
	query := "SELECT COUNT(*) FROM growth_event WHERE user_id=? AND kind=?"
	args := []any{a.User, kind}
	if period == "day" {
		query += " AND business_day=?"
		args = append(args, BusinessDay(now))
	}
	if period == "month" {
		query += " AND business_month=?"
		args = append(args, BusinessMonth(now))
	}
	var count int64
	if e = tx.QueryRowCtx(ctx, &count, query, args...); e != nil {
		return event, e
	}
	if count >= limit {
		if !recordLimit {
			return event, ErrLimit
		}
		amount = 0
	}
	if kind == "learning" || kind == "note" || kind == "activity" || kind == "review" {
		var used int64
		e = tx.QueryRowCtx(ctx, &used, "SELECT COALESCE(SUM(original_award),0) FROM growth_event WHERE user_id=? AND business_month=? AND kind IN ('learning','note','activity','review')", a.User, BusinessMonth(now))
		if e != nil {
			return event, e
		}
		if amount > MonthlyGrowthLimit-used {
			amount = MonthlyGrowthLimit - used
		}
		if amount < 0 {
			amount = 0
		}
	}
	result, e := tx.ExecCtx(ctx, "INSERT INTO growth_event(event_key,user_id,kind,reference_no,original_award,current_award,state,business_day,business_month,rule_version,created_at,updated_at) VALUES(?,?,?,?,?,?,'posted',?,?,?,?,?)", key, a.User, kind, ref, amount, amount, BusinessDay(now), BusinessMonth(now), RuleVersion, now.Unix(), now.Unix())
	if e != nil {
		return event, e
	}
	id, e := result.LastInsertId()
	if e != nil {
		return event, e
	}
	event = Event{ID: id, User: a.User, Key: key, Kind: kind, Reference: ref, Original: amount, Current: amount, State: "posted", Version: RuleVersion, Created: now.Unix()}
	e = s.applyEvent(ctx, tx, a, &event, amount, "posted")
	return event, e
}
func (s *Store) applyEvent(ctx context.Context, tx sqlx.Session, a *Account, e *Event, target int64, state string) error {
	delta := target - e.Applied
	if target < 0 || a.Total+delta < 0 {
		return fmt.Errorf("invalid growth reversal")
	}
	if delta != 0 {
		newTotal := a.Total + delta
		key := fmt.Sprintf("event:%d:to:%d:revision:%d", e.ID, target, a.Revision+1)
		kind := e.Kind
		if delta < 0 {
			kind = "reversal"
		}
		_, err := tx.ExecCtx(ctx, "INSERT INTO growth_ledger(event_key,user_id,source_id,kind,reference_no,delta,total_after,rule_version,created_at) VALUES(?,?,?,?,?,?,?,?,?)", key, a.User, e.ID, kind, e.Reference, delta, newTotal, e.Version, s.now().Unix())
		if err != nil {
			return err
		}
		_, err = tx.ExecCtx(ctx, "UPDATE growth_account SET total=?,revision=revision+1,updated_at=? WHERE user_id=?", newTotal, s.now().Unix(), a.User)
		if err != nil {
			return err
		}
		a.Total = newTotal
		a.Revision++
		if err = s.syncCoupons(ctx, tx, *a); err != nil {
			return err
		}
	}
	_, err := tx.ExecCtx(ctx, "UPDATE growth_event SET current_award=?,applied=?,state=?,updated_at=? WHERE id=?", target, target, state, s.now().Unix(), e.ID)
	e.Applied = target
	e.Current = target
	e.State = state
	return err
}
func (s *Store) Profile(ctx context.Context, user string) (event Event, err error) {
	err = s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		a, e := lockAccount(ctx, tx, user, s.now().Unix())
		if e != nil {
			return e
		}
		var valid int
		e = tx.QueryRowCtx(ctx, &valid, "SELECT COUNT(*) FROM askxuan_user.user WHERE id=? AND TRIM(nickname)<>'' AND TRIM(avatar)<>'' AND status=1", user)
		if e != nil {
			return e
		}
		if valid != 1 {
			return common.NewBizError(40943, "请先保存昵称与头像，可使用系统头像")
		}
		event, e = s.awardTask(ctx, tx, &a, "profile", "once", 20, 1, "lifetime")
		return e
	})
	return event, err
}
func (s *Store) SaveNote(ctx context.Context, user, action, body string, remove bool) error {
	if len([]rune(action)) > 120 || len([]rune(body)) > 2000 || (!remove && strings.TrimSpace(action) == "") {
		return common.ErrParam
	}
	now := s.now()
	return s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		a, e := lockAccount(ctx, tx, user, now.Unix())
		if e != nil {
			return e
		}
		if remove {
			_, e = tx.ExecCtx(ctx, "UPDATE growth_note SET action='',body='',deleted=1,updated_at=? WHERE user_id=? AND business_day=?", now.Unix(), user, BusinessDay(now))
			return e
		}
		_, e = tx.ExecCtx(ctx, "INSERT INTO growth_note(user_id,business_day,action,body,updated_at) VALUES(?,?,?,?,?) ON DUPLICATE KEY UPDATE action=VALUES(action),body=VALUES(body),deleted=0,updated_at=VALUES(updated_at)", user, BusinessDay(now), strings.TrimSpace(action), body, now.Unix())
		if e != nil {
			return e
		}
		_, e = s.awardTask(ctx, tx, &a, "note", BusinessDay(now), 2, 1, "day")
		return e
	})
}
func (s *Store) Preferences(ctx context.Context, user string, visible bool) error {
	return s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		_, e := lockAccount(ctx, tx, user, s.now().Unix())
		if e != nil {
			return e
		}
		_, e = tx.ExecCtx(ctx, "UPDATE growth_account SET badge_visible=?,revision=revision+1,updated_at=? WHERE user_id=?", visible, s.now().Unix(), user)
		return e
	})
}
func audit(ctx context.Context, tx sqlx.Session, operator, action, reference string, value any, now int64) error {
	raw, e := json.Marshal(value)
	if e != nil {
		return e
	}
	_, e = tx.ExecCtx(ctx, "INSERT INTO growth_audit(operator_id,action,reference_no,detail_json,created_at) VALUES(?,?,?,?,?)", operator, action, reference, string(raw), now)
	return e
}

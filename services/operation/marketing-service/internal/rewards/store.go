// Package rewards owns platform-funded campaigns. Participation debits the existing
// points ledger in the same MySQL transaction as the participation code.
package rewards

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

var ErrInvalid = errors.New("活动参数不正确，请检查奖品、预算、名额与截止时间")
var ErrClosed = errors.New("活动未开始、已截止或名额已满")
var ErrConflict = errors.New("状态已变化，请刷新；已发布的活动规则不能修改")
var ErrNotFound = errors.New("记录不存在或无权操作")
var ErrBalance = errors.New("积分不足，请返回积分页面查看余额")
var ErrPrice = errors.New("请确认本期参与积分后重试")

type Store struct {
	DB     *sql.DB
	Random io.Reader
}
type Campaign struct {
	PublishedAt      int64  `json:"publishedAt"`
	ID               int64  `json:"id"`
	Title            string `json:"title"`
	Kind             string `json:"kind"`
	PrizeName        string `json:"prizeName"`
	Image            string `json:"image"`
	Description      string `json:"description"`
	Rules            string `json:"rules"`
	PrizeValue       int64  `json:"prizeValue"`
	Budget           int64  `json:"budget"`
	PointsCost       int64  `json:"pointsCost"`
	PrizeQuantity    int    `json:"prizeQuantity"`
	Capacity         int    `json:"capacity"`
	ParticipantCount int    `json:"participantCount"`
	AwardedCount     int    `json:"awardedCount"`
	StartsAt         int64  `json:"startsAt"`
	EndsAt           int64  `json:"endsAt"`
	Status           string `json:"status"`
	Version          int    `json:"version"`
	DrawnAt          int64  `json:"drawnAt"`
	PoolDigest       string `json:"poolDigest"`
	Announcement     string `json:"announcement"`
	CreatedAt        int64  `json:"createdAt"`
	Phase            string `json:"phase"`
	Algorithm        string `json:"algorithm"`
}
type Entry struct {
	ID          int64  `json:"id"`
	CampaignID  int64  `json:"campaignId"`
	UserID      string `json:"-"`
	Code        string `json:"code"`
	Outcome     string `json:"outcome"`
	CreatedAt   int64  `json:"createdAt"`
	Title       string `json:"title"`
	PointsSpent int64  `json:"pointsSpent"`
}
type Order struct {
	ID          int64  `json:"id"`
	CampaignID  int64  `json:"campaignId"`
	EntryID     int64  `json:"entryId"`
	UserID      string `json:"-"`
	PrizeName   string `json:"prizeName"`
	Code        string `json:"code"`
	Status      string `json:"status"`
	Receiver    string `json:"receiver"`
	Mobile      string `json:"mobile"`
	Address     string `json:"address"`
	Carrier     string `json:"carrier"`
	TrackingNo  string `json:"trackingNo"`
	CreatedAt   int64  `json:"createdAt"`
	ClaimedAt   int64  `json:"claimedAt"`
	ShippedAt   int64  `json:"shippedAt"`
	CompletedAt int64  `json:"completedAt"`
}
type Detail struct {
	Campaign      Campaign `json:"campaign"`
	Mine          *Entry   `json:"mine"`
	Winners       []Entry  `json:"winners"`
	PointsBalance int64    `json:"pointsBalance"`
}
type Audit struct {
	ID         int64  `json:"id"`
	CampaignID int64  `json:"campaignId"`
	OrderID    int64  `json:"orderId"`
	Actor      string `json:"actor"`
	Action     string `json:"action"`
	Detail     string `json:"detail"`
	CreatedAt  int64  `json:"createdAt"`
}
type OrderAction struct {
	Receiver   string `json:"receiver"`
	Mobile     string `json:"mobile"`
	Address    string `json:"address"`
	Carrier    string `json:"carrier"`
	TrackingNo string `json:"trackingNo"`
}

const campaignCols = `id,title,kind,prize_name,image,description,rules,prize_value,budget,prize_quantity,capacity,participant_count,awarded_count,starts_at,ends_at,status,version,drawn_at,pool_digest,announcement,created_at,published_at,points_cost`
const entryCols = `id,campaign_id,user_id,code,outcome,created_at,points_spent`
const orderCols = `id,campaign_id,entry_id,user_id,prize_name,code,status,receiver,mobile,address,carrier,tracking_no,created_at,claimed_at,shipped_at,completed_at`

type scanner interface{ Scan(...any) error }

func scanCampaign(r scanner) (c Campaign, err error) {
	err = r.Scan(&c.ID, &c.Title, &c.Kind, &c.PrizeName, &c.Image, &c.Description, &c.Rules, &c.PrizeValue, &c.Budget, &c.PrizeQuantity, &c.Capacity, &c.ParticipantCount, &c.AwardedCount, &c.StartsAt, &c.EndsAt, &c.Status, &c.Version, &c.DrawnAt, &c.PoolDigest, &c.Announcement, &c.CreatedAt, &c.PublishedAt, &c.PointsCost)
	c.Phase = c.phase(time.Now().Unix())
	c.Algorithm = "crypto-rand-uniform-v1"
	return
}
func scanEntry(r scanner) (e Entry, err error) {
	err = r.Scan(&e.ID, &e.CampaignID, &e.UserID, &e.Code, &e.Outcome, &e.CreatedAt, &e.PointsSpent)
	return
}
func scanOrder(r scanner) (o Order, err error) {
	err = r.Scan(&o.ID, &o.CampaignID, &o.EntryID, &o.UserID, &o.PrizeName, &o.Code, &o.Status, &o.Receiver, &o.Mobile, &o.Address, &o.Carrier, &o.TrackingNo, &o.CreatedAt, &o.ClaimedAt, &o.ShippedAt, &o.CompletedAt)
	return
}
func (c Campaign) phase(now int64) string {
	if c.Status != "published" {
		return c.Status
	}
	if now < c.StartsAt {
		return "scheduled"
	}
	if now >= c.EndsAt {
		return "awaiting_draw"
	}
	if c.ParticipantCount >= c.Capacity {
		return "full"
	}
	return "open"
}
func validText(s string, min, max int) bool {
	n := utf8.RuneCountInString(strings.TrimSpace(s))
	return n >= min && n <= max
}
func Validate(c Campaign, now int64) error {
	if c.PointsCost < 1 || c.PointsCost > 100000000 {
		return ErrInvalid
	}
	if !validText(c.Title, 1, 120) || !validText(c.PrizeName, 1, 120) || !validText(c.Description, 1, 10000) || !validText(c.Rules, 1, 10000) || (c.Kind != "pool" && c.Kind != "wheel") || c.PrizeValue < 1 || c.PrizeValue > 100000000 || c.PrizeQuantity < 1 || c.PrizeQuantity > 1000 || c.Capacity < c.PrizeQuantity || c.Capacity > 100000 || c.Budget < c.PrizeValue*int64(c.PrizeQuantity) || c.Budget > 100000000000 || c.StartsAt < 1 || c.EndsAt <= c.StartsAt || c.EndsAt <= now || c.EndsAt-c.StartsAt > 366*86400 {
		return ErrInvalid
	}
	if c.Image != "" {
		u, e := url.Parse(c.Image)
		if e != nil || u.Scheme != "https" || u.Host == "" || len(c.Image) > 1000 {
			return ErrInvalid
		}
	}
	return nil
}
func (s Store) uniform(n int) (int, error) {
	if n <= 0 {
		return 0, ErrInvalid
	}
	r := s.Random
	if r == nil {
		r = rand.Reader
	}
	v, e := rand.Int(r, big.NewInt(int64(n)))
	if e != nil {
		return 0, e
	}
	return int(v.Int64()), nil
}
func audit(ctx context.Context, tx *sql.Tx, cid, oid int64, actor, action, detail string) error {
	_, e := tx.ExecContext(ctx, `INSERT INTO reward_audit(campaign_id,order_id,actor,action,detail,created_at) VALUES(?,?,?,?,?,?)`, cid, oid, actor, action, detail, time.Now().Unix())
	return e
}
func (s Store) transaction(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, e := s.DB.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	if e = fn(tx); e != nil {
		return e
	}
	return tx.Commit()
}
func lockedCampaign(ctx context.Context, tx *sql.Tx, id int64) (Campaign, error) {
	c, e := scanCampaign(tx.QueryRowContext(ctx, "SELECT "+campaignCols+" FROM reward_campaign WHERE id=? FOR UPDATE", id))
	if errors.Is(e, sql.ErrNoRows) {
		e = ErrNotFound
	}
	return c, e
}
func (s Store) Save(ctx context.Context, c Campaign, actor string) (Campaign, error) {
	if e := Validate(c, time.Now().Unix()); e != nil {
		return c, e
	}
	e := s.transaction(ctx, func(tx *sql.Tx) error {
		if c.ID == 0 {
			r, e := tx.ExecContext(ctx, `INSERT INTO reward_campaign(title,kind,prize_name,image,description,rules,prize_value,budget,prize_quantity,capacity,starts_at,ends_at,announcement,created_at,points_cost) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, c.Title, c.Kind, c.PrizeName, c.Image, c.Description, c.Rules, c.PrizeValue, c.Budget, c.PrizeQuantity, c.Capacity, c.StartsAt, c.EndsAt, "", time.Now().Unix(), c.PointsCost)
			if e != nil {
				return e
			}
			c.ID, e = r.LastInsertId()
			if e != nil {
				return e
			}
		} else {
			old, e := lockedCampaign(ctx, tx, c.ID)
			if e != nil {
				return e
			}
			if old.Status != "draft" || c.Version != old.Version {
				return ErrConflict
			}
			_, e = tx.ExecContext(ctx, `UPDATE reward_campaign SET title=?,kind=?,prize_name=?,image=?,description=?,rules=?,prize_value=?,budget=?,prize_quantity=?,capacity=?,starts_at=?,ends_at=?,points_cost=?,version=version+1 WHERE id=?`, c.Title, c.Kind, c.PrizeName, c.Image, c.Description, c.Rules, c.PrizeValue, c.Budget, c.PrizeQuantity, c.Capacity, c.StartsAt, c.EndsAt, c.PointsCost, c.ID)
			if e != nil {
				return e
			}
		}
		return audit(ctx, tx, c.ID, 0, actor, "save_draft", fmt.Sprintf("平台预算独立配置；每人每期 %d 积分", c.PointsCost))
	})
	if e != nil {
		return c, e
	}
	return s.campaign(ctx, c.ID, true)
}
func (s Store) campaign(ctx context.Context, id int64, admin bool) (Campaign, error) {
	c, e := scanCampaign(s.DB.QueryRowContext(ctx, "SELECT "+campaignCols+" FROM reward_campaign WHERE id=?", id))
	if errors.Is(e, sql.ErrNoRows) || (e == nil && !admin && c.PublishedAt == 0) {
		e = ErrNotFound
	}
	return c, e
}
func (s Store) List(ctx context.Context, admin bool, page int, kind string) ([]Campaign, error) {
	q := "SELECT " + campaignCols + " FROM reward_campaign WHERE 1=1"
	args := []any{}
	if !admin {
		q += " AND published_at>0"
	}
	if kind != "" {
		q += " AND kind=?"
		args = append(args, kind)
	}
	q += " ORDER BY id DESC LIMIT 20 OFFSET ?"
	args = append(args, (page-1)*20)
	rows, e := s.DB.QueryContext(ctx, q, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Campaign{}
	for rows.Next() {
		c, e := scanCampaign(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (s Store) Detail(ctx context.Context, id int64, user string, admin bool) (Detail, error) {
	d := Detail{Winners: []Entry{}}
	tx, e := s.DB.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if e != nil {
		return d, e
	}
	defer tx.Rollback()
	c, e := scanCampaign(tx.QueryRowContext(ctx, "SELECT "+campaignCols+" FROM reward_campaign WHERE id=?", id))
	if errors.Is(e, sql.ErrNoRows) || (e == nil && !admin && c.PublishedAt == 0) {
		e = ErrNotFound
	}
	if e != nil {
		return d, e
	}
	d.Campaign = c
	if !admin && user != "" {
		e = tx.QueryRowContext(ctx, "SELECT balance FROM askxuan_payment.points_account WHERE user_id=?", user).Scan(&d.PointsBalance)
		if e != nil && !errors.Is(e, sql.ErrNoRows) {
			return d, e
		}
	}
	if user != "" {
		m, e := scanEntry(tx.QueryRowContext(ctx, "SELECT "+entryCols+" FROM reward_entry WHERE campaign_id=? AND user_id=?", id, user))
		if e == nil {
			d.Mine = &m
		} else if !errors.Is(e, sql.ErrNoRows) {
			return d, e
		}
	}
	rows, e := tx.QueryContext(ctx, "SELECT "+entryCols+" FROM reward_entry WHERE campaign_id=? AND outcome='won' ORDER BY id LIMIT 1000", id)
	if e != nil {
		return d, e
	}
	defer rows.Close()
	for rows.Next() {
		w, e := scanEntry(rows)
		if e != nil {
			return d, e
		}
		d.Winners = append(d.Winners, w)
	}
	if err := rows.Err(); err != nil {
		return d, err
	}
	rows.Close()
	return d, tx.Commit()
}
func (s Store) Publish(ctx context.Context, id int64, actor string) error {
	return s.transaction(ctx, func(tx *sql.Tx) error {
		c, e := lockedCampaign(ctx, tx, id)
		if e != nil {
			return e
		}
		if c.Status == "published" {
			return nil
		}
		if c.Status != "draft" {
			return ErrConflict
		}
		if e = Validate(c, time.Now().Unix()); e != nil {
			return e
		}
		_, e = tx.ExecContext(ctx, "UPDATE reward_campaign SET status='published',published_at=?,version=version+1 WHERE id=?", time.Now().Unix(), id)
		if e != nil {
			return e
		}
		return audit(ctx, tx, id, 0, actor, "publish", fmt.Sprintf("规则冻结，每人每期 %d 积分", c.PointsCost))
	})
}
func (s Store) Cancel(ctx context.Context, id int64, actor, reason string) error {
	if !validText(reason, 2, 500) {
		return ErrInvalid
	}
	return s.transaction(ctx, func(tx *sql.Tx) error {
		c, e := lockedCampaign(ctx, tx, id)
		if e != nil {
			return e
		}
		if c.Status == "cancelled" {
			return nil
		}
		if c.ParticipantCount != 0 || (c.Status != "draft" && c.Status != "published") {
			return ErrConflict
		}
		_, e = tx.ExecContext(ctx, "UPDATE reward_campaign SET status='cancelled',announcement=?,version=version+1 WHERE id=?", reason, id)
		if e != nil {
			return e
		}
		return audit(ctx, tx, id, 0, actor, "cancel", reason)
	})
}
func newOrder(ctx context.Context, tx *sql.Tx, c Campaign, e Entry) error {
	r, err := tx.ExecContext(ctx, `INSERT INTO reward_order(campaign_id,entry_id,user_id,prize_name,code,created_at) VALUES(?,?,?,?,?,?)`, c.ID, e.ID, e.UserID, c.PrizeName, e.Code, time.Now().Unix())
	if err != nil {
		return err
	}
	oid, err := r.LastInsertId()
	if err != nil {
		return err
	}
	return audit(ctx, tx, c.ID, oid, "system", "award", e.Code)
}
func (s Store) Join(ctx context.Context, id int64, user string, expectedPoints int64) (Entry, error) {
	var entry Entry
	if user == "" || user == "0" {
		return entry, ErrNotFound
	}
	err := s.transaction(ctx, func(tx *sql.Tx) error {
		c, e := lockedCampaign(ctx, tx, id)
		if e != nil {
			return e
		}
		old, e := scanEntry(tx.QueryRowContext(ctx, "SELECT "+entryCols+" FROM reward_entry WHERE campaign_id=? AND user_id=?", id, user))
		if e == nil {
			entry = old
			return nil
		}
		if !errors.Is(e, sql.ErrNoRows) {
			return e
		}
		if c.phase(time.Now().Unix()) != "open" {
			return ErrClosed
		}
		if c.PointsCost < 1 || expectedPoints != c.PointsCost {
			return ErrPrice
		}
		entry = Entry{CampaignID: id, UserID: user, Code: fmt.Sprintf("WX%08d-%06d", id, c.ParticipantCount+1), Outcome: "pending", CreatedAt: time.Now().Unix(), PointsSpent: c.PointsCost}
		// The existing account lock also serializes redemption, payment refunds and
		// joins to other campaigns. Missing accounts have no spendable points.
		var balance int64
		e = tx.QueryRowContext(ctx, "SELECT balance FROM askxuan_payment.points_account WHERE user_id=? FOR UPDATE", user).Scan(&balance)
		if errors.Is(e, sql.ErrNoRows) {
			return ErrBalance
		}
		if e != nil {
			return e
		}
		if balance < c.PointsCost {
			return ErrBalance
		}
		if _, e = tx.ExecContext(ctx, "UPDATE askxuan_payment.points_account SET balance=balance-? WHERE user_id=?", c.PointsCost, user); e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO askxuan_payment.points_ledger(user_id,event_key,kind,delta,balance_after,reference_no) VALUES(?,?,?,?,?,?)`, user, fmt.Sprintf("reward:%d:%s", id, user), "reward_"+c.Kind, -c.PointsCost, balance-c.PointsCost, entry.Code); e != nil {
			return e
		}
		if c.Kind == "wheel" {
			n, e := s.uniform(c.Capacity - c.ParticipantCount)
			if e != nil {
				return e
			}
			entry.Outcome = "lost"
			if n < c.PrizeQuantity-c.AwardedCount {
				entry.Outcome = "won"
			}
		}
		r, e := tx.ExecContext(ctx, `INSERT INTO reward_entry(campaign_id,user_id,code,outcome,created_at,points_spent) VALUES(?,?,?,?,?,?)`, id, user, entry.Code, entry.Outcome, entry.CreatedAt, entry.PointsSpent)
		if e != nil {
			return e
		}
		entry.ID, e = r.LastInsertId()
		if e != nil {
			return e
		}
		won := 0
		if entry.Outcome == "won" {
			won = 1
			if e = newOrder(ctx, tx, c, entry); e != nil {
				return e
			}
		}
		_, e = tx.ExecContext(ctx, "UPDATE reward_campaign SET participant_count=participant_count+1,awarded_count=awarded_count+?,version=version+1 WHERE id=?", won, id)
		if e != nil {
			return e
		}
		return audit(ctx, tx, id, 0, user, "join", fmt.Sprintf("%s:%s；扣除 %d 积分", entry.Code, entry.Outcome, entry.PointsSpent))
	})
	return entry, err
}
func (s Store) Draw(ctx context.Context, id int64) error {
	return s.transaction(ctx, func(tx *sql.Tx) error {
		c, e := lockedCampaign(ctx, tx, id)
		if e != nil {
			return e
		}
		if c.Status == "drawn" {
			return nil
		}
		if c.Status != "published" || time.Now().Unix() < c.EndsAt {
			return ErrClosed
		}
		rows, e := tx.QueryContext(ctx, "SELECT "+entryCols+" FROM reward_entry WHERE campaign_id=? ORDER BY id", id)
		if e != nil {
			return e
		}
		entries := []Entry{}
		h := sha256.New()
		for rows.Next() {
			v, e := scanEntry(rows)
			if e != nil {
				rows.Close()
				return e
			}
			entries = append(entries, v)
			fmt.Fprintln(h, v.Code)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return e
		}
		if len(entries) != c.ParticipantCount {
			return errors.New("reward participant count inconsistent")
		}
		winners := c.AwardedCount
		if c.Kind == "pool" {
			winners = c.PrizeQuantity
			if winners > len(entries) {
				winners = len(entries)
			}
			// Partial Fisher-Yates: every size-K subset is equally likely; no replacement.
			for i := 0; i < winners; i++ {
				j, e := s.uniform(len(entries) - i)
				if e != nil {
					return e
				}
				entries[i], entries[i+j] = entries[i+j], entries[i]
			}
			if _, e = tx.ExecContext(ctx, "UPDATE reward_entry SET outcome='lost' WHERE campaign_id=?", id); e != nil {
				return e
			}
			for _, v := range entries[:winners] {
				if _, e = tx.ExecContext(ctx, "UPDATE reward_entry SET outcome='won' WHERE id=?", v.ID); e != nil {
					return e
				}
				if e = newOrder(ctx, tx, c, v); e != nil {
					return e
				}
			}
		}
		announcement := fmt.Sprintf("活动已结束。有效参与 %d 人，实际中奖 %d 人；每人参与消耗 %d 积分，奖品由平台预算提供。", len(entries), winners, c.PointsCost)
		if len(entries) == 0 {
			announcement = "活动已到截止时间，本期无人参与，未发放奖品。"
		}
		_, e = tx.ExecContext(ctx, "UPDATE reward_campaign SET status='drawn',awarded_count=?,drawn_at=?,pool_digest=?,announcement=?,version=version+1 WHERE id=?", winners, time.Now().Unix(), hex.EncodeToString(h.Sum(nil)), announcement, id)
		if e != nil {
			return e
		}
		return audit(ctx, tx, id, 0, "system", "draw", announcement)
	})
}
func (s Store) DrawDue(ctx context.Context) error {
	rows, e := s.DB.QueryContext(ctx, "SELECT id FROM reward_campaign WHERE status='published' AND ends_at<=? ORDER BY ends_at LIMIT 100", time.Now().Unix())
	if e != nil {
		return e
	}
	ids := []int64{}
	for rows.Next() {
		var id int64
		if e = rows.Scan(&id); e != nil {
			rows.Close()
			return e
		}
		ids = append(ids, id)
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	var errs []error
	for _, id := range ids {
		if e = s.Draw(ctx, id); e != nil && !errors.Is(e, ErrClosed) {
			errs = append(errs, fmt.Errorf("draw campaign %d: %w", id, e))
		}
	}
	return errors.Join(errs...)
}
func (s Store) Entries(ctx context.Context, user string, page int) ([]Entry, error) {
	rows, e := s.DB.QueryContext(ctx, `SELECT e.id,e.campaign_id,e.user_id,e.code,e.outcome,e.created_at,c.title,e.points_spent FROM reward_entry e JOIN reward_campaign c ON c.id=e.campaign_id WHERE e.user_id=? ORDER BY e.id DESC LIMIT 20 OFFSET ?`, user, (page-1)*20)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Entry{}
	for rows.Next() {
		var v Entry
		if e = rows.Scan(&v.ID, &v.CampaignID, &v.UserID, &v.Code, &v.Outcome, &v.CreatedAt, &v.Title, &v.PointsSpent); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s Store) Orders(ctx context.Context, user string, admin bool, page int, status string) ([]Order, error) {
	q := "SELECT " + orderCols + " FROM reward_order WHERE 1=1"
	args := []any{}
	if !admin {
		q += " AND user_id=?"
		args = append(args, user)
	}
	if status != "" {
		q += " AND status=?"
		args = append(args, status)
	}
	q += " ORDER BY id DESC LIMIT 20 OFFSET ?"
	args = append(args, (page-1)*20)
	rows, e := s.DB.QueryContext(ctx, q, args...)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Order{}
	for rows.Next() {
		v, e := scanOrder(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func (s Store) OrderAction(ctx context.Context, id int64, user, action string, a OrderAction) error {
	return s.transaction(ctx, func(tx *sql.Tx) error {
		o, e := scanOrder(tx.QueryRowContext(ctx, "SELECT "+orderCols+" FROM reward_order WHERE id=? FOR UPDATE", id))
		if errors.Is(e, sql.ErrNoRows) {
			return ErrNotFound
		}
		if e != nil {
			return e
		}
		if action != "ship" && o.UserID != user {
			return ErrNotFound
		}
		switch action {
		case "claim":
			if !validText(a.Receiver, 1, 80) || !validText(a.Mobile, 6, 32) || !validText(a.Address, 5, 600) {
				return ErrInvalid
			}
			if o.Status != "awaiting_address" {
				if o.Receiver == a.Receiver && o.Mobile == a.Mobile && o.Address == a.Address {
					return nil
				}
				return ErrConflict
			}
			_, e = tx.ExecContext(ctx, "UPDATE reward_order SET receiver=?,mobile=?,address=?,status='pending',claimed_at=? WHERE id=?", a.Receiver, a.Mobile, a.Address, time.Now().Unix(), id)
		case "ship":
			if !validText(a.Carrier, 1, 80) || !validText(a.TrackingNo, 3, 100) {
				return ErrInvalid
			}
			if o.Status == "shipped" || o.Status == "completed" {
				if o.Carrier == a.Carrier && o.TrackingNo == a.TrackingNo {
					return nil
				}
				return ErrConflict
			}
			if o.Status != "pending" {
				return ErrConflict
			}
			_, e = tx.ExecContext(ctx, "UPDATE reward_order SET carrier=?,tracking_no=?,status='shipped',shipped_at=? WHERE id=?", a.Carrier, a.TrackingNo, time.Now().Unix(), id)
		case "complete":
			if o.Status == "completed" {
				return nil
			}
			if o.Status != "shipped" {
				return ErrConflict
			}
			_, e = tx.ExecContext(ctx, "UPDATE reward_order SET status='completed',completed_at=? WHERE id=?", time.Now().Unix(), id)
		default:
			return ErrInvalid
		}
		if e != nil {
			return e
		}
		return audit(ctx, tx, o.CampaignID, id, user, action, "奖品履约状态更新")
	})
}
func (s Store) Audits(ctx context.Context, id int64, page int) ([]Audit, error) {
	rows, e := s.DB.QueryContext(ctx, "SELECT id,campaign_id,order_id,actor,action,detail,created_at FROM reward_audit WHERE campaign_id=? ORDER BY id DESC LIMIT 20 OFFSET ?", id, (page-1)*20)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Audit{}
	for rows.Next() {
		var v Audit
		if e = rows.Scan(&v.ID, &v.CampaignID, &v.OrderID, &v.Actor, &v.Action, &v.Detail, &v.CreatedAt); e != nil {
			return nil, e
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

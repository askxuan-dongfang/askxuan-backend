package growth

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// RuleConfig is the complete adjustable arithmetic contract. Eligibility,
// currency, Asia/Shanghai day boundaries and non-stacking remain invariants.
// Published configurations are immutable; old orders keep their full snapshot.
type RuleConfig struct {
	Levels               []Level `json:"levels"`
	Tasks                []Task  `json:"tasks"`
	MonthlyGrowthLimit   int64   `json:"monthlyGrowthLimit"`
	MonthlyDiscountLimit int64   `json:"monthlyDiscountLimitCents"`
	GoodsCap             int64   `json:"goodsDiscountCapCents"`
	ServiceCap           int64   `json:"serviceDiscountCapCents"`
	ObservationDays      int64   `json:"observationDays"`
	CouponDays           int64   `json:"couponDays"`
}

func DefaultRules() RuleConfig {
	return RuleConfig{Levels: Levels(), Tasks: []Task{
		{Kind: "profile", Award: 20, Limit: 1, Period: "lifetime"},
		{Kind: "learning", Award: 3, Limit: 1, Period: "day"},
		{Kind: "note", Award: 2, Limit: 1, Period: "day"},
		{Kind: "activity", Award: 20, Limit: 2, Period: "month"},
		{Kind: "review", Award: 5, Limit: 4, Period: "month"},
	}, MonthlyGrowthLimit: 200, MonthlyDiscountLimit: 30000, GoodsCap: 10000, ServiceCap: 5000, ObservationDays: 7, CouponDays: 30}
}
func (r RuleConfig) Tier(total int64) Level {
	for i := len(r.Levels) - 1; i >= 0; i-- {
		if total >= r.Levels[i].Threshold {
			return r.Levels[i]
		}
	}
	return r.Levels[0]
}
func (r RuleConfig) Validate() error {
	if len(r.Levels) != 5 || len(r.Tasks) != 5 || r.MonthlyGrowthLimit < 1 || r.MonthlyGrowthLimit > 10000 || r.MonthlyDiscountLimit < 0 || r.MonthlyDiscountLimit > MonthlyDiscountLimit || r.GoodsCap < 0 || r.GoodsCap > MaxAmount || r.ServiceCap < 0 || r.ServiceCap > MaxAmount || r.ObservationDays != 7 || r.CouponDays != 30 {
		return common.ErrParam
	}
	for i, l := range r.Levels {
		if l.Code != i+1 || strings.TrimSpace(l.Name) == "" || len([]rune(l.Name)) > 20 || strings.TrimSpace(l.Benefit) == "" || len([]rune(l.Benefit)) > 120 || l.Threshold < 0 || l.Threshold > 10000000 || l.GoodsRate < 5000 || l.GoodsRate > 10000 || l.ServiceRate < 5000 || l.ServiceRate > 10000 || l.CouponMinimum < l.CouponValue || l.CouponMinimum > MaxAmount || l.CouponValue < 0 || l.CouponValue > 10000 {
			return common.ErrParam
		}
		if i == 0 && (l.Threshold != 0 || l.GoodsRate != 10000 || l.ServiceRate != 10000 || l.CouponValue != 0) {
			return common.ErrParam
		}
		if i > 0 && (l.Threshold <= r.Levels[i-1].Threshold || l.GoodsRate > r.Levels[i-1].GoodsRate || l.ServiceRate > r.Levels[i-1].ServiceRate) {
			return common.ErrParam
		}
	}
	for i, t := range r.Tasks {
		d := DefaultRules().Tasks[i]
		if t.Kind != d.Kind || t.Period != d.Period || t.Award < 1 || t.Award > 200 || t.Limit < 1 || t.Limit > 31 || (t.Period != "month" && t.Limit != 1) || t.Used != 0 || t.Available {
			return common.ErrParam
		}
	}
	return nil
}

type RuleRelease struct {
	Version     string     `db:"version" json:"version"`
	EffectiveAt int64      `db:"effective_at" json:"effectiveAt"`
	Summary     string     `db:"summary" json:"summary"`
	Raw         string     `db:"config_json" json:"-"`
	Config      RuleConfig `db:"-" json:"config"`
	Status      string     `db:"status" json:"status"`
	CreatedAt   int64      `db:"created_at" json:"createdAt"`
}

const ruleFields = "version,effective_at,summary,config_json,status,created_at"

func ruleAt(ctx context.Context, q reader, at int64) (r RuleRelease, err error) {
	err = q.QueryRowPartialCtx(ctx, &r, "SELECT "+ruleFields+" FROM growth_rule_release WHERE status='published' AND effective_at<=? ORDER BY effective_at DESC,version DESC LIMIT 1", at)
	if err != nil {
		return
	}
	err = json.Unmarshal([]byte(r.Raw), &r.Config)
	if err == nil {
		err = r.Config.Validate()
	}
	return
}
func (s *Store) Rules(ctx context.Context, page int, admin bool) (map[string]any, error) {
	if page < 1 || page > 10000 {
		return nil, common.ErrParam
	}
	rows := []RuleRelease{}
	where := "status='published' AND effective_at<=?"
	args := []any{s.now().Unix()}
	if admin {
		where = "1=1"
		args = nil
	}
	args = append(args, (page-1)*30)
	if e := s.DB.QueryRowsPartialCtx(ctx, &rows, "SELECT "+ruleFields+" FROM growth_rule_release WHERE "+where+" ORDER BY effective_at DESC,version DESC LIMIT 31 OFFSET ?", args...); e != nil {
		return nil, e
	}
	more := len(rows) > 30
	if more {
		rows = rows[:30]
	}
	current, e := ruleAt(ctx, s.DB, s.now().Unix())
	if e != nil {
		return nil, e
	}
	for i := range rows {
		if e = json.Unmarshal([]byte(rows[i].Raw), &rows[i].Config); e != nil {
			return nil, e
		}
		if rows[i].Status == "published" {
			switch {
			case rows[i].EffectiveAt > s.now().Unix():
				rows[i].Status = "scheduled"
			case rows[i].Version == current.Version:
				rows[i].Status = "current"
			default:
				rows[i].Status = "historical"
			}
		}
	}
	return map[string]any{"items": rows, "hasMore": more, "currentVersion": current.Version, "serverTime": s.now().Unix()}, nil
}

type RuleProposal struct {
	Version     string     `json:"version"`
	BaseVersion string     `json:"baseVersion"`
	EffectiveAt int64      `json:"effectiveAt"`
	Summary     string     `json:"summary"`
	Config      RuleConfig `json:"config"`
}

func (s *Store) validateRule(p RuleProposal) error {
	if p.EffectiveAt < s.now().Unix()+60 || p.EffectiveAt > s.now().Add(366*24*time.Hour).Unix() {
		return common.NewBizError(40944, "生效时间须在一分钟后至一年内；过期申请请重新提交")
	}

	if !regexp.MustCompile(`^growth-[a-zA-Z0-9][a-zA-Z0-9._-]{0,23}$`).MatchString(p.Version) || p.Version == RuleVersion || p.BaseVersion == "" || len([]rune(strings.TrimSpace(p.Summary))) < 5 || len([]rune(p.Summary)) > 500 {
		return common.ErrParam
	}
	return p.Config.Validate()
}
func (s *Store) RulePreview(ctx context.Context, p RuleProposal) (map[string]any, error) {
	if e := s.validateRule(p); e != nil {
		return nil, e
	}
	current, e := ruleAt(ctx, s.DB, s.now().Unix())
	if e != nil {
		return nil, e
	}
	if p.BaseVersion != current.Version {
		return nil, ErrChanged
	}
	// Explicit samples are arithmetic previews only, never a budget allocation.
	samples := []map[string]any{}
	lines := []Line{{ID: "goods", Amount: 20000, Category: "goods", Eligible: true, GrowthEligible: true}, {ID: "service", Amount: 10000, Category: "service", Eligible: true, GrowthEligible: true}, {ID: "excluded", Amount: 3000, Category: "excluded"}}
	for _, total := range []int64{0, 200, 1000, 3000, 8000} {
		before, e := candidatesWithRules(lines, current.Config.Tier(total), current.Config.MonthlyDiscountLimit, nil, current.Config)
		if e != nil {
			return nil, e
		}
		after, e := candidatesWithRules(lines, p.Config.Tier(total), p.Config.MonthlyDiscountLimit, nil, p.Config)
		if e != nil {
			return nil, e
		}
		samples = append(samples, map[string]any{"total": total, "beforeLevel": current.Config.Tier(total), "afterLevel": p.Config.Tier(total), "before": before[0], "after": after[0]})
	}
	return map[string]any{"proposal": p, "current": current, "samples": samples}, nil
}
func (s *Store) publishRule(ctx context.Context, tx sqlx.Session, p RuleProposal, operator string) error {
	if e := s.validateRule(p); e != nil {
		return e
	}
	// Serialize schedule changes with the settings row. Reading rules never
	// mutates state, so activation is exact even after a worker outage.
	var revision int64
	if e := tx.QueryRowCtx(ctx, &revision, "SELECT revision FROM growth_settings WHERE id=1 FOR UPDATE"); e != nil {
		return e
	}
	current, e := ruleAt(ctx, tx, s.now().Unix())
	if e != nil {
		return e
	}
	if current.Version != p.BaseVersion {
		return ErrChanged
	}
	var n int64
	if e = tx.QueryRowCtx(ctx, &n, "SELECT COUNT(*) FROM growth_rule_release WHERE version=? OR (status='published' AND effective_at>?)", p.Version, s.now().Unix()); e != nil {
		return e
	}
	if n > 0 {
		return ErrChanged
	}
	raw, e := json.Marshal(p.Config)
	if e != nil {
		return e
	}
	_, e = tx.ExecCtx(ctx, "INSERT INTO growth_rule_release(version,effective_at,summary,config_json,status,created_at,approved_by) VALUES(?,?,?,?,'published',?,?)", p.Version, p.EffectiveAt, strings.TrimSpace(p.Summary), string(raw), s.now().Unix(), operator)
	return e
}
func (s *Store) cancelRule(ctx context.Context, tx sqlx.Session, version string) error {
	var revision int64
	if e := tx.QueryRowCtx(ctx, &revision, "SELECT revision FROM growth_settings WHERE id=1 FOR UPDATE"); e != nil {
		return e
	}
	r, e := tx.ExecCtx(ctx, "UPDATE growth_rule_release SET status='cancelled' WHERE version=? AND status='published' AND effective_at>?", version, s.now().Unix())
	if e != nil {
		return e
	}
	n, e := r.RowsAffected()
	if e == nil && n != 1 {
		return ErrChanged
	}
	return e
}

type LevelNotice struct {
	Level    Level `json:"level"`
	Previous int   `json:"previousLevel"`
	Revision int64 `json:"revision"`
}

// Claim is at-most-once across devices, not a browser-local toast flag. A
// downgrade updates the observed level without manufacturing an upgrade.
func (s *Store) ClaimLevelNotice(ctx context.Context, user string) (notice *LevelNotice, err error) {
	err = s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		a, e := lockAccount(ctx, tx, user, s.now().Unix())
		if e != nil {
			return e
		}
		r, e := ruleAt(ctx, tx, s.now().Unix())
		if e != nil {
			return e
		}
		level := r.Config.Tier(a.Total)
		var previous int
		e = tx.QueryRowCtx(ctx, &previous, "SELECT level FROM growth_level_notice WHERE user_id=?", user)
		if errors.Is(e, sqlx.ErrNotFound) {
			previous = 1
		} else if e != nil {
			return e
		}
		if level.Code > previous {
			notice = &LevelNotice{Level: level, Previous: previous, Revision: a.Revision}
		}
		_, e = tx.ExecCtx(ctx, "INSERT INTO growth_level_notice(user_id,level,updated_at) VALUES(?,?,?) ON DUPLICATE KEY UPDATE level=VALUES(level),updated_at=VALUES(updated_at)", user, level.Code, s.now().Unix())
		return e
	})
	return
}

func (s *Store) reconcileRuleAccounts(ctx context.Context) error {
	r, e := ruleAt(ctx, s.DB, s.now().Unix())
	if e != nil {
		return e
	}
	users := []string{}
	if e = s.DB.QueryRowsCtx(ctx, &users, "SELECT a.user_id FROM growth_account a LEFT JOIN growth_account_rule v ON v.user_id=a.user_id WHERE v.version IS NULL OR v.version<>? ORDER BY a.user_id LIMIT 300", r.Version); e != nil {
		return e
	}
	for _, user := range users {
		if e = s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
			a, e := lockAccount(ctx, tx, user, s.now().Unix())
			if e != nil {
				return e
			}
			if e = s.syncCoupons(ctx, tx, a); e != nil {
				return e
			}
			_, e = tx.ExecCtx(ctx, "INSERT INTO growth_account_rule(user_id,version) VALUES(?,?) ON DUPLICATE KEY UPDATE version=VALUES(version)", user, r.Version)
			return e
		}); e != nil {
			return e
		}
	}
	return nil
}

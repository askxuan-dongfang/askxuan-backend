package growth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"net/url"
	"strconv"
	"strings"
)

type Proposal struct {
	ID         int64  `db:"id" json:"id"`
	Kind       string `db:"kind" json:"kind"`
	Payload    string `db:"payload_json" json:"payload"`
	Reason     string `db:"reason" json:"reason"`
	Status     string `db:"status" json:"status"`
	CreatedBy  string `db:"created_by" json:"createdBy"`
	ApprovedBy string `db:"approved_by" json:"approvedBy"`
	Created    int64  `db:"created_at" json:"createdAt"`
	Approved   int64  `db:"approved_at" json:"approvedAt"`
}

const proposalFields = "id,kind,payload_json,reason,status,created_by,approved_by,created_at,approved_at"

type Adjustment struct {
	User     string `json:"userId"`
	SourceID int64  `json:"sourceEventId"`
	Amount   int64  `json:"amount"`
}

func (s *Store) Dashboard(ctx context.Context) (map[string]any, error) {
	cfg, e := s.Settings(ctx)
	if e != nil {
		return nil, e
	}
	budgets := []Budget{}
	policies := []Policy{}
	proposals := []Proposal{}
	if e = s.DB.QueryRowsCtx(ctx, &budgets, "SELECT kind,total_cents,held_cents,spent_cents,revision FROM growth_budget ORDER BY kind"); e != nil {
		return nil, e
	}
	if e = s.DB.QueryRowsCtx(ctx, &policies, "SELECT "+policyFields+" FROM growth_item_policy ORDER BY updated_at DESC LIMIT 200"); e != nil {
		return nil, e
	}
	if e = s.DB.QueryRowsCtx(ctx, &proposals, "SELECT "+proposalFields+" FROM growth_proposal ORDER BY id DESC LIMIT 100"); e != nil {
		return nil, e
	}
	release, e := ruleAt(ctx, s.DB, s.now().Unix())
	if e != nil {
		return nil, e
	}
	history, e := s.Rules(ctx, 1, true)
	if e != nil {
		return nil, e
	}
	return map[string]any{"rule": release, "ruleHistory": history, "settings": cfg, "levels": release.Config.Levels, "budgets": budgets, "policies": policies, "proposals": proposals}, nil
}

// Resolve an item from the catalog, never from caller-supplied owner IDs.
func policyOwner(ctx context.Context, q reader, p Policy) (Policy, error) {
	if len(p.ID) < 1 || len(p.ID) > 96 {
		return p, common.ErrParam
	}
	var owner struct {
		Title string `db:"title"`
		Type  string `db:"owner_type"`
		ID    string `db:"owner_id"`
	}
	query := ""
	args := []any{p.ID}
	switch p.Type {
	case "shop_sku":
		query = "SELECT p.name title,'platform' owner_type,'' owner_id FROM askxuan_product.product_sku s JOIN askxuan_product.product p ON p.id=s.product_id WHERE s.id=?"
	case "material_sku":
		query = "SELECT m.name title,'platform' owner_type,'' owner_id FROM askxuan_diy.material_sku s JOIN askxuan_diy.material m ON m.id=s.material_id WHERE s.id=?"
	case "material":
		query = "SELECT name title,'platform' owner_type,'' owner_id FROM askxuan_diy.material WHERE id=?"
	case "temple_service", "master_service":
		parts := strings.Split(p.ID, ":")
		if len(parts) != 2 {
			return p, common.ErrParam
		}
		args = []any{parts[0], parts[1]}
		if p.Type == "temple_service" {
			query = "SELECT service_code title,'temple' owner_type,temple_code owner_id FROM askxuan_temple.temple_service WHERE temple_code=? AND service_code=?"
		} else {
			query = "SELECT service_code title,'master' owner_type,master_code owner_id FROM askxuan_master.master_service_tag WHERE master_code=? AND service_code=?"
		}
	case "consultation":
		query = "SELECT dharma_name title,'master' owner_type,code owner_id FROM askxuan_master.master WHERE code=?"
	case "ai_report":
		query = "SELECT title,'platform' owner_type,'' owner_id FROM askxuan_ai.ai_report_product WHERE code=?"
	default:
		return p, common.ErrParam
	}
	if e := q.QueryRowCtx(ctx, &owner, query, args...); e != nil {
		return p, e
	}
	p.Title = owner.Title
	p.OwnerType = owner.Type
	p.OwnerID = owner.ID
	return p, nil
}
func (s *Store) Propose(ctx context.Context, operator, kind, reason string, payload json.RawMessage) (id int64, e error) {
	if len([]rune(strings.TrimSpace(reason))) < 5 || len([]rune(reason)) > 500 || len(payload) > 8192 {
		return 0, common.ErrParam
	}
	var normalized any
	switch kind {
	case "rules":
		var p RuleProposal
		if json.Unmarshal(payload, &p) != nil {
			return 0, common.ErrParam
		}
		if e = s.validateRule(p); e != nil {
			return 0, e
		}
		normalized = p
	case "rule_cancel":
		var p struct {
			Version string `json:"version"`
		}
		if json.Unmarshal(payload, &p) != nil || p.Version == "" || p.Version == RuleVersion {
			return 0, common.ErrParam
		}
		normalized = p
	case "settings":
		var p Settings
		if json.Unmarshal(payload, &p) != nil {
			return 0, common.ErrParam
		}
		if p.Revision < 1 {
			return 0, common.ErrParam
		}
		normalized = struct {
			Enabled   bool  `json:"enabled"`
			Discounts bool  `json:"discountsEnabled"`
			Revision  int64 `json:"revision"`
		}{p.Enabled, p.Discounts, p.Revision}
	case "budget":
		var p Budget
		if json.Unmarshal(payload, &p) != nil || (p.Kind != "level" && p.Kind != "coupon") || p.Total < 0 || p.Total > MaxAmount || p.Revision < 1 {
			return 0, common.ErrParam
		}
		normalized = p
	case "policy":
		var p Policy
		if json.Unmarshal(payload, &p) != nil || p.Cost < 0 || p.Minimum < 0 || p.Cap < 0 || p.Cost > MaxAmount || p.Minimum > MaxAmount || p.Cap > MaxAmount || p.Revision < 0 {
			return 0, common.ErrParam
		}
		p, e = policyOwner(ctx, s.DB, p)
		if e != nil {
			return 0, e
		}
		p.Consent = p.OwnerType == "platform"
		normalized = p
	case "adjustment":
		var p Adjustment
		if json.Unmarshal(payload, &p) != nil || p.User == "" || len(p.User) > 64 || p.Amount < 0 || p.Amount > 200 || p.SourceID < 0 || (p.SourceID == 0 && p.Amount == 0) || (p.SourceID > 0 && p.Amount != 0) {
			return 0, common.ErrParam
		}
		normalized = p
	default:
		return 0, common.ErrParam
	}
	raw, e := json.Marshal(normalized)
	if e != nil {
		return 0, e
	}
	e = s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		res, err := tx.ExecCtx(ctx, "INSERT INTO growth_proposal(kind,payload_json,reason,created_by,created_at) VALUES(?,?,?,?,?)", kind, string(raw), strings.TrimSpace(reason), operator, s.now().Unix())
		if err != nil {
			return err
		}
		id, err = res.LastInsertId()
		if err != nil {
			return err
		}
		return audit(ctx, tx, operator, "proposal", fmt.Sprint(id), normalized, s.now().Unix())
	})
	return
}
func (s *Store) Approve(ctx context.Context, operator string, id int64, approve bool) error {
	return s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var p Proposal
		e := tx.QueryRowCtx(ctx, &p, "SELECT "+proposalFields+" FROM growth_proposal WHERE id=? FOR UPDATE", id)
		if e != nil {
			return e
		}
		if p.CreatedBy == operator {
			return common.NewBizError(40340, "配置与补偿需由另一位管理员复核")
		}
		if p.Status != "draft" {
			return ErrChanged
		}
		status := "rejected"
		if approve {
			status = "approved"
			switch p.Kind {
			case "rules":
				var v RuleProposal
				if e = json.Unmarshal([]byte(p.Payload), &v); e != nil {
					return e
				}
				e = s.publishRule(ctx, tx, v, operator)
			case "rule_cancel":
				var v struct {
					Version string `json:"version"`
				}
				if e = json.Unmarshal([]byte(p.Payload), &v); e != nil {
					return e
				}
				e = s.cancelRule(ctx, tx, v.Version)
			case "settings":
				var v Settings
				if e = json.Unmarshal([]byte(p.Payload), &v); e != nil {
					return e
				}
				res, err := tx.ExecCtx(ctx, "UPDATE growth_settings SET enabled=?,discounts_enabled=?,revision=revision+1,updated_by=?,updated_at=? WHERE id=1 AND revision=?", v.Enabled, v.Discounts, operator, s.now().Unix(), v.Revision)
				if err != nil {
					return err
				}
				n, err := res.RowsAffected()
				if err != nil {
					return err
				}
				if n != 1 {
					return ErrChanged
				}
			case "budget":
				var v Budget
				if e = json.Unmarshal([]byte(p.Payload), &v); e != nil {
					return e
				}
				b, err := lockBudget(ctx, tx, v.Kind)
				if err != nil {
					return err
				}
				if b.Revision != v.Revision || v.Total < b.Held+b.Spent {
					return ErrChanged
				}
				_, e = tx.ExecCtx(ctx, "UPDATE growth_budget SET total_cents=?,revision=revision+1 WHERE kind=?", v.Total, v.Kind)
			case "policy":
				var v Policy
				if e = json.Unmarshal([]byte(p.Payload), &v); e != nil {
					return e
				}
				current, err := policyOwner(ctx, tx, v)
				if err != nil {
					return err
				}
				if current.OwnerType != v.OwnerType || current.OwnerID != v.OwnerID {
					return ErrChanged
				}
				var old Policy
				err = tx.QueryRowCtx(ctx, &old, "SELECT "+policyFields+" FROM growth_item_policy WHERE item_type=? AND item_id=? FOR UPDATE", v.Type, v.ID)
				if errors.Is(err, sqlx.ErrNotFound) {
					if v.Revision != 0 {
						return ErrChanged
					}
					_, e = tx.ExecCtx(ctx, "INSERT INTO growth_item_policy(item_type,item_id,title,owner_type,owner_id,enabled,consent,cost_cents,minimum_contribution_cents,subsidy_cap_cents,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)", v.Type, v.ID, v.Title, v.OwnerType, v.OwnerID, v.Enabled, v.OwnerType == "platform", v.Cost, v.Minimum, v.Cap, s.now().Unix())
				} else if err != nil {
					return err
				} else {
					if v.Revision != old.Revision {
						return ErrChanged
					}
					_, e = tx.ExecCtx(ctx, "UPDATE growth_item_policy SET title=?,enabled=?,cost_cents=?,minimum_contribution_cents=?,subsidy_cap_cents=?,revision=revision+1,updated_at=? WHERE item_type=? AND item_id=?", v.Title, v.Enabled, v.Cost, v.Minimum, v.Cap, s.now().Unix(), v.Type, v.ID)
				}
			case "adjustment":
				var v Adjustment
				if e = json.Unmarshal([]byte(p.Payload), &v); e != nil {
					return e
				}
				a, err := lockAccount(ctx, tx, v.User, s.now().Unix())
				if err != nil {
					return err
				}
				if v.SourceID > 0 {
					var event Event
					if e = tx.QueryRowCtx(ctx, &event, "SELECT "+eventFields+" FROM growth_event WHERE id=? AND user_id=?", v.SourceID, v.User); e != nil {
						return e
					}
					if e = s.applyEvent(ctx, tx, &a, &event, 0, "revoked"); e != nil {
						return e
					}
					_, e = tx.ExecCtx(ctx, "UPDATE growth_event SET reason='verified_correction' WHERE id=?", event.ID)
				} else {
					_, e = s.awardTask(ctx, tx, &a, "adjustment", fmt.Sprint(p.ID), v.Amount, 1000, "lifetime")
				}
			default:
				return common.ErrParam
			}
			if e != nil {
				return e
			}
		}
		if _, e = tx.ExecCtx(ctx, "UPDATE growth_proposal SET status=?,approved_by=?,approved_at=? WHERE id=?", status, operator, s.now().Unix(), id); e != nil {
			return e
		}
		return audit(ctx, tx, operator, status, fmt.Sprint(id), p, s.now().Unix())
	})
}
func (s *Store) SaveContent(ctx context.Context, operator string, c Content) (id int64, e error) {
	if len([]rune(strings.TrimSpace(c.Title))) < 2 || len([]rune(c.Title)) > 120 || len(c.Body) > 60000 || (c.Kind != "article" && c.Kind != "media") {
		return 0, common.ErrParam
	}
	if c.Kind == "article" && len([]rune(strings.TrimSpace(c.Body))) < 100 {
		return 0, common.ErrParam
	}
	if c.Kind == "media" {
		u, err := url.Parse(c.MediaURL)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || len(c.MediaURL) > 1000 || c.Duration < 60 || c.Duration > 7200 {
			return 0, common.ErrParam
		}
	}
	e = s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		if c.ID == 0 {
			res, err := tx.ExecCtx(ctx, "INSERT INTO growth_content(title,kind,body,media_url,duration_seconds,enabled,updated_by,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)", c.Title, c.Kind, c.Body, c.MediaURL, c.Duration, c.Enabled, operator, s.now().Unix(), s.now().Unix())
			if err != nil {
				return err
			}
			id, err = res.LastInsertId()
			if err != nil {
				return err
			}
		} else {
			res, err := tx.ExecCtx(ctx, "UPDATE growth_content SET title=?,kind=?,body=?,media_url=?,duration_seconds=?,enabled=?,revision=revision+1,updated_by=?,updated_at=? WHERE id=? AND revision=?", c.Title, c.Kind, c.Body, c.MediaURL, c.Duration, c.Enabled, operator, s.now().Unix(), c.ID, c.Revision)
			if err != nil {
				return err
			}
			n, err := res.RowsAffected()
			if err != nil {
				return err
			}
			if n != 1 {
				return ErrChanged
			}
			id = c.ID
		}
		return audit(ctx, tx, operator, "content", fmt.Sprint(id), map[string]any{"title": c.Title, "enabled": c.Enabled, "revision": c.Revision + 1}, s.now().Unix())
	})
	return
}
func (s *Store) SaveActivity(ctx context.Context, operator string, a Activity) (id int64, e error) {
	if len([]rune(a.Title)) < 2 || len([]rune(a.Title)) > 120 || len([]rune(a.Description)) > 2000 || a.Starts < 1 || a.Ends < a.Starts {
		return 0, common.ErrParam
	}
	e = s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		if a.ID == 0 {
			res, err := tx.ExecCtx(ctx, "INSERT INTO growth_activity(title,description,starts_at,ends_at,enabled,created_by,created_at) VALUES(?,?,?,?,?,?,?)", a.Title, a.Description, a.Starts, a.Ends, a.Enabled, operator, s.now().Unix())
			if err != nil {
				return err
			}
			id, err = res.LastInsertId()
			if err != nil {
				return err
			}
		} else {
			_, err := tx.ExecCtx(ctx, "UPDATE growth_activity SET title=?,description=?,starts_at=?,ends_at=?,enabled=? WHERE id=?", a.Title, a.Description, a.Starts, a.Ends, a.Enabled, a.ID)
			if err != nil {
				return err
			}
			id = a.ID
		}
		return audit(ctx, tx, operator, "activity", fmt.Sprint(id), a, s.now().Unix())
	})
	return
}
func (s *Store) ReplySupport(ctx context.Context, operator string, id int64, reply string, urgency int) error {
	if len([]rune(reply)) < 2 || len([]rune(reply)) > 2000 || urgency < 0 || urgency > 2 {
		return common.ErrParam
	}
	return s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		res, e := tx.ExecCtx(ctx, "UPDATE growth_support SET reply=?,urgency=?,status='replied',updated_at=? WHERE id=?", reply, urgency, s.now().Unix(), id)
		if e != nil {
			return e
		}
		n, e := res.RowsAffected()
		if e != nil {
			return e
		}
		if n != 1 {
			return ErrChanged
		}
		return audit(ctx, tx, operator, "support_reply", fmt.Sprint(id), map[string]any{"urgency": urgency}, s.now().Unix())
	})
}
func (s *Store) ProviderIdentity(ctx context.Context, kind string, id int64, code string) (string, error) {
	if kind == "temple" && code != "" {
		var verified string
		e := s.DB.QueryRowCtx(ctx, &verified, "SELECT code FROM askxuan_temple.temple WHERE id=? AND code=?", id, code)
		return verified, e
	}
	if kind == "master" && id > 0 {
		var verified string
		e := s.DB.QueryRowCtx(ctx, &verified, "SELECT code FROM askxuan_master.master WHERE id=?", id)
		return verified, e
	}
	return "", common.ErrForbidden
}
func (s *Store) ProviderPolicies(ctx context.Context, kind, id string) (v []Policy, e error) {
	v = []Policy{}
	e = s.DB.QueryRowsCtx(ctx, &v, "SELECT "+policyFields+" FROM growth_item_policy WHERE owner_type=? AND owner_id=? ORDER BY item_type,item_id", kind, id)
	return
}
func (s *Store) Consent(ctx context.Context, operator, ownerType, ownerID, itemType, itemID string, consent bool, revision int64) error {
	return s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		res, e := tx.ExecCtx(ctx, "UPDATE growth_item_policy SET consent=?,revision=revision+1,updated_at=? WHERE owner_type=? AND owner_id=? AND item_type=? AND item_id=? AND revision=?", consent, s.now().Unix(), ownerType, ownerID, itemType, itemID, revision)
		if e != nil {
			return e
		}
		n, e := res.RowsAffected()
		if e != nil {
			return e
		}
		if n != 1 {
			return ErrChanged
		}
		return audit(ctx, tx, operator, "provider_consent", itemType+":"+itemID, map[string]any{"consent": consent, "ownerType": ownerType, "ownerId": ownerID}, s.now().Unix())
	})
}

type Bill struct {
	OrderType    string `json:"orderType"`
	OrderNo      string `json:"orderNo"`
	Status       string `json:"status"`
	Contract     int64  `json:"contractGrossCents"`
	CustomerPaid int64  `json:"customerPaidCents"`
	Subsidy      int64  `json:"platformSubsidyCents"`
	Lines        []Line `json:"lines"`
	Created      int64  `json:"createdAt"`
}

func (s *Store) ProviderBills(ctx context.Context, kind, id string) (v []Bill, e error) {
	v = []Bill{}
	quotes := []PriceQuote{}
	e = s.DB.QueryRowsPartialCtx(ctx, &quotes, "SELECT "+quoteFields+" FROM growth_quote WHERE status IN ('paid','refunded') AND JSON_CONTAINS(snapshot_json,JSON_OBJECT('ownerType',?,'ownerId',?),'$.selected.lines') ORDER BY created_at DESC LIMIT 100", kind, id)
	if e != nil {
		return v, e
	}
	for _, q := range quotes {
		if e = json.Unmarshal([]byte(q.Raw), &q.Snapshot); e != nil {
			return nil, e
		}
		b := Bill{OrderType: q.Kind, OrderNo: q.No, Status: q.Status, Created: q.Created, Lines: []Line{}}
		for _, l := range q.Snapshot.Selected.Lines {
			if l.OwnerType == kind && l.OwnerID == id {
				b.Contract += l.Amount
				b.CustomerPaid += l.Net
				b.Subsidy += l.Discount
				b.Lines = append(b.Lines, l)
			}
		}
		if len(b.Lines) > 0 {
			v = append(v, b)
		}
	}
	return
}
func validPage(raw string) (int, error) {
	if raw == "" {
		return 1, nil
	}
	n, e := strconv.Atoi(raw)
	if e != nil || n < 1 || n > 100000 {
		return 0, common.ErrParam
	}
	return n, nil
}

// Audit excludes private journals and learning content bodies.
func (s *Store) Audit(ctx context.Context) (v []struct {
	ID       int64  `db:"id" json:"id"`
	Operator string `db:"operator_id" json:"operatorId"`
	Action   string `db:"action" json:"action"`
	Ref      string `db:"reference_no" json:"referenceNo"`
	Detail   string `db:"detail_json" json:"detail"`
	Created  int64  `db:"created_at" json:"createdAt"`
}, e error) {
	e = s.DB.QueryRowsCtx(ctx, &v, "SELECT id,operator_id,action,reference_no,detail_json,created_at FROM growth_audit ORDER BY id DESC LIMIT 200")
	if v == nil {
		v = make([]struct {
			ID       int64  `db:"id" json:"id"`
			Operator string `db:"operator_id" json:"operatorId"`
			Action   string `db:"action" json:"action"`
			Ref      string `db:"reference_no" json:"referenceNo"`
			Detail   string `db:"detail_json" json:"detail"`
			Created  int64  `db:"created_at" json:"createdAt"`
		}, 0)
	}
	return
}

// AccountAudit exposes accounting evidence only, never private note text.
func (s *Store) AccountAudit(ctx context.Context, user string, page int) (map[string]any, error) {
	if user == "" || len(user) > 64 || page < 1 || page > 100000 {
		return nil, common.ErrParam
	}
	var account Account
	err := s.DB.QueryRowCtx(ctx, &account, "SELECT user_id,total,revision,badge_visible,updated_at FROM growth_account WHERE user_id=?", user)
	if errors.Is(err, sqlx.ErrNotFound) {
		return map[string]any{"exists": false, "events": []Event{}}, nil
	}
	if err != nil {
		return nil, err
	}
	events := []Event{}
	if err = s.DB.QueryRowsCtx(ctx, &events, "SELECT "+eventFields+" FROM growth_event WHERE user_id=? ORDER BY id DESC LIMIT 31 OFFSET ?", user, (page-1)*30); err != nil {
		return nil, err
	}
	more := len(events) > 30
	if more {
		events = events[:30]
	}
	release, err := ruleAt(ctx, s.DB, s.now().Unix())
	if err != nil {
		return nil, err
	}
	return map[string]any{"exists": true, "userId": user, "total": account.Total, "level": release.Config.Tier(account.Total), "events": events, "hasMore": more}, nil
}

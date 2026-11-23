// Package catalogtranslation provides the authoritative English catalog read for
// customer discovery and transaction validation.
package catalogtranslation

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const MaxIDs = 100

var ErrLookup = errors.New("invalid catalog translation lookup")

const templeVisible = "t.auth_status='certified' AND t.platform_status='normal' AND t.shelf_status='on_shelf'"

type sourceSpec struct {
	from, key, alias, owner, visible, ready, stamp, fields string
}

// These projections contain customer-facing text only. Eligibility stays tied
// to the original catalog, including the parent and publication guards.
var sources = map[string]sourceSpec{
	"shop-product": {
		from: "askxuan_product.product s LEFT JOIN askxuan_logistics.freight_template ft ON ft.id=s.freight_template_id",
		key:  "s.id", visible: "s.status='on_shelf'",
		ready: `NOT EXISTS (SELECT 1 FROM askxuan_product.product_sku sku WHERE sku.product_id=s.id
			AND (TRIM(sku.spec_name)<>'' OR TRIM(sku.spec_value)<>''))
			AND (s.freight_template_id=0 OR (ft.status='enabled' AND (ft.free_shipping=1 OR ft.type='by_piece')))`,
		stamp:  "SHA2(JSON_OBJECT('id',s.freight_template_id,'type',ft.type,'free',ft.free_shipping,'status',ft.status,'config',ft.config),256)",
		fields: "JSON_OBJECT('title',s.name,'description',COALESCE(s.description,''))",
	},
	"points-product": {
		from: "askxuan_payment.points_product s", key: "s.id", visible: "s.status='on_sale'", ready: "1",
		fields: "JSON_OBJECT('title',s.name,'subtitle',s.category,'description',s.description)",
	},
	"reward-campaign": {
		from: "askxuan_marketing.reward_campaign s", key: "s.id", visible: "s.published_at>0",
		ready:  "TRIM(s.rules)<>''",
		fields: "JSON_OBJECT('title',s.title,'description',s.description,'rules',s.rules,'prizeName',s.prize_name)",
	},
	"reward-prize": {
		from: "askxuan_marketing.reward_prize s JOIN askxuan_marketing.reward_campaign c ON c.id=s.campaign_id",
		key:  "s.id", visible: "c.published_at>0", ready: "s.kind='physical'",
		fields: "JSON_OBJECT('title',s.name)",
	},
	"diy-material": {
		from: "askxuan_diy.material s", key: "s.id", visible: "s.status='on_shelf'",
		fields: "JSON_OBJECT('title',s.name,'subtitle',s.spec)",
	},
	"diy-design": {
		from: "askxuan_diy.catalog_public_design_text s", key: "s.id",
		stamp: "CAST(s.publication_id AS CHAR)", visible: "1",
		fields: "JSON_OBJECT('title',s.title,'description',s.description)",
	},
	"temple": {
		from: "askxuan_temple.temple t", key: "t.id", alias: "t.code", owner: "t.code", visible: templeVisible,
		fields: "JSON_OBJECT('title',t.name,'description',t.description,'address',t.address)",
	},
	"master": {
		from: "askxuan_master.master s LEFT JOIN askxuan_temple.temple t ON t.code=s.temple_code",
		key:  "s.id", alias: "s.code", owner: "IF(s.manage_by='temple',s.temple_code,'')",
		visible: "s.auth_status='已认证' AND s.shelf_status='on_shelf' AND s.platform_status='normal' " +
			"AND (s.temple_code='' OR (t.auth_status='certified' AND t.platform_status='normal'))",
		fields: "JSON_OBJECT('title',COALESCE(NULLIF(TRIM(s.dharma_name),''),s.lay_name),'subtitle',s.position,'specialties',s.specialties)",
	},
	"temple-service": {
		from: "askxuan_temple.temple_service s JOIN askxuan_temple.temple t ON t.code=s.temple_code",
		key:  "s.id", owner: "s.temple_code", visible: "s.status='on_shelf' AND " + templeVisible,
		fields: "JSON_OBJECT('title',s.service_name)",
	},
	"belief": {
		from: "askxuan_temple.belief_profile s", key: "s.code", visible: "s.status='enabled'",
		fields: "JSON_OBJECT('title',s.name,'subtitle',s.summary,'description',s.description)",
	},
}

var beliefCode = regexp.MustCompile(`^[a-z][a-z0-9_]{2,31}$`)
var providerCode = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]{0,31}$`)

type Source struct {
	ID             string
	OwnerTemple    string
	PublicEligible bool
	Revision       string
	Fields         map[string]string
}

type Published struct {
	Fields           map[string]string `json:"fields"`
	TransactionReady bool              `json:"transactionReady"`
	Revision         string            `json:"revision"`
}

type sourceRow struct {
	ID             string `db:"resource_id"`
	Alias          string `db:"resource_alias"`
	Owner          string `db:"owner_temple"`
	PublicEligible bool   `db:"public_eligible"`
	Stamp          string `db:"source_stamp"`
	Fields         string `db:"source_fields"`
}

type publicRow struct {
	ID               string `db:"resource_id"`
	Alias            string `db:"resource_alias"`
	Stamp            string `db:"source_stamp"`
	Fields           string `db:"source_fields"`
	SourceRevision   string `db:"source_revision"`
	PublishedFields  string `db:"published_fields"`
	TransactionReady bool   `db:"transaction_ready"`
}

type Queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func Load(ctx context.Context, db sqlx.Session, kind, id string) (Source, error) {
	query, args, err := statement(kind, []string{id}, false)
	if err != nil {
		return Source{}, err
	}
	var row sourceRow
	if err = db.QueryRowCtx(ctx, &row, query, args...); err != nil {
		return Source{}, err
	}
	if id != row.ID && id != row.Alias {
		return Source{}, sqlx.ErrNotFound
	}
	fields, revision, err := fieldsRevision(row.Fields, row.Stamp)
	return Source{ID: row.ID, OwnerTemple: row.Owner, PublicEligible: row.PublicEligible, Revision: revision, Fields: fields}, err
}

func Lookup(ctx context.Context, db Queryer, kind string, ids []string) (map[string]Published, error) {
	query, args, err := statement(kind, ids, true)
	if err != nil {
		return nil, err
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []publicRow
	for rows.Next() {
		var row publicRow
		if err := rows.Scan(&row.ID, &row.Alias, &row.Stamp, &row.Fields, &row.SourceRevision, &row.PublishedFields, &row.TransactionReady); err != nil {
			return nil, err
		}
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return publishedItems(ids, items)
}

func LookupSession(ctx context.Context, db sqlx.Session, kind string, ids []string) (map[string]Published, error) {
	query, args, err := statement(kind, ids, true)
	if err != nil {
		return nil, err
	}
	var rows []publicRow
	if err = db.QueryRowsCtx(ctx, &rows, query, args...); err != nil {
		return nil, err
	}
	return publishedItems(ids, rows)
}

func statement(kind string, ids []string, published bool) (string, []any, error) {
	spec, ok := sources[kind]
	if !ok || len(ids) == 0 || len(ids) > MaxIDs {
		return "", nil, ErrLookup
	}
	var keys, aliases []any
	for _, id := range ids {
		switch {
		case kind == "belief" && beliefCode.MatchString(id):
			keys = append(keys, id)
		case spec.alias != "" && providerCode.MatchString(id):
			aliases = append(aliases, id)
		default:
			n, err := strconv.ParseInt(id, 10, 64)
			if err != nil || n <= 0 || kind == "belief" || strconv.FormatInt(n, 10) != id {
				return "", nil, ErrLookup
			}
			keys = append(keys, n)
		}
	}
	var matches []string
	if len(keys) > 0 {
		matches = append(matches, spec.key+" IN ("+placeholders(len(keys))+")")
	}
	if len(aliases) > 0 {
		matches = append(matches, spec.alias+" IN ("+placeholders(len(aliases))+")")
	}
	predicate := "(" + strings.Join(matches, " OR ") + ")"
	args := append(keys, aliases...)
	if published {
		predicate += " AND (" + spec.visible + ")"
	}
	key := "CAST(" + spec.key + " AS CHAR)"
	alias := spec.alias
	if alias == "" {
		alias = "''"
	}
	stamp := spec.stamp
	if stamp == "" {
		stamp = "''"
	}
	columns := key + " AS resource_id," + alias + " AS resource_alias,"
	textColumns := stamp + " AS source_stamp," + spec.fields + " AS source_fields"
	from := spec.from
	if published {
		from += " JOIN askxuan_ai.catalog_translation tr ON tr.kind=? AND tr.resource_id=" + key +
			" AND tr.published_fields IS NOT NULL AND tr.published_source_revision IS NOT NULL"
		args = append([]any{kind}, args...)
		ready := spec.ready
		if ready == "" {
			ready = "0"
		}
		columns += textColumns + ",tr.published_source_revision AS source_revision,tr.published_fields,IF(" + ready + ",1,0) AS transaction_ready"
	} else {
		owner := spec.owner
		if owner == "" {
			owner = "''"
		}
		columns += owner + " AS owner_temple,IF(" + spec.visible + ",1,0) AS public_eligible," + textColumns
	}
	return "SELECT " + columns + " FROM " + from + " WHERE " + predicate, args, nil
}

func placeholders(length int) string {
	return strings.Repeat("?,", length-1) + "?"
}

func publishedItems(ids []string, rows []publicRow) (map[string]Published, error) {
	requested := make(map[string]bool, len(ids))
	for _, id := range ids {
		requested[id] = true
	}
	items := make(map[string]Published)
	for _, row := range rows {
		source, current, err := fieldsRevision(row.Fields, row.Stamp)
		if err != nil {
			return nil, err
		}
		if row.SourceRevision != current || source["title"] == "" {
			continue
		}
		var en map[string]string
		if err := json.Unmarshal([]byte(row.PublishedFields), &en); err != nil {
			return nil, fmt.Errorf("decode published catalog fields: %w", err)
		}
		revision, err := digest(current, en)
		if err != nil {
			return nil, err
		}
		value := Published{Fields: en, TransactionReady: row.TransactionReady, Revision: revision}
		for _, key := range []string{row.ID, row.Alias} {
			if requested[key] {
				items[key] = value
			}
		}
	}
	return items, nil
}

func fieldsRevision(raw, stamp string) (map[string]string, string, error) {
	var fields map[string]string
	if err := json.Unmarshal([]byte(raw), &fields); err != nil {
		return nil, "", fmt.Errorf("decode source catalog fields: %w", err)
	}
	for key, text := range fields {
		if text = strings.TrimSpace(text); text == "" {
			delete(fields, key)
		} else {
			fields[key] = text
		}
	}
	revision, err := digest(stamp, fields)
	return fields, revision, err
}

func digest(stamp string, fields map[string]string) (string, error) {
	body, err := json.Marshal(fields)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(append([]byte(stamp+"\x00"), body...))
	return hex.EncodeToString(sum[:]), nil
}

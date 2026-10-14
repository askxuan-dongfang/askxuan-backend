package model

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// Expressions are internal aliases, never request values. All three consumers use
// the same rule resolution: explicit empty is custom and must NOT inherit.
func DefaultIntentionTagsSQL(service string) string {
	return "COALESCE((SELECT GROUP_CONCAT(r.tag_code ORDER BY r.tag_code) FROM service_intention_rule r JOIN intent_tag i ON i.code=r.tag_code AND i.status='enabled' WHERE r.service_code=" + service + "),'')"
}
func TempleIntentionTagsSQL(alias string) string {
	return "CASE WHEN " + alias + ".intent_mode='custom' THEN COALESCE((SELECT GROUP_CONCAT(b.tag_code ORDER BY b.tag_code) FROM askxuan_temple.temple_service_intent_tag b JOIN intent_tag i ON i.code=b.tag_code AND i.status='enabled' WHERE b.temple_service_id=" + alias + ".id),'') ELSE " + DefaultIntentionTagsSQL(alias+".service_code") + " END"
}
func MasterIntentionTagsSQL(alias string) string {
	return "CASE WHEN " + alias + ".intent_mode='custom' THEN COALESCE((SELECT GROUP_CONCAT(b.tag_code ORDER BY b.tag_code) FROM master_service_intention b JOIN intent_tag i ON i.code=b.tag_code AND i.status='enabled' WHERE b.master_code=" + alias + ".master_code AND b.service_code=" + alias + ".service_code),'') ELSE " + DefaultIntentionTagsSQL(alias+".service_code") + " END"
}

var regexpService = regexp.MustCompile(`^S[0-9]{3}$`)

type ServiceIntentionUpdate struct {
	Kind    string   `json:"kind"`
	ID      string   `json:"id"`
	Service string   `json:"serviceCode"`
	Tags    []string `json:"tags"`
	Mode    string   `json:"mode,optional"`
}

func (r *ServiceIntentionUpdate) Validate() error {
	if r.Mode == "" {
		r.Mode = "custom"
	} // old clients submit explicit selections
	if r.Mode != "inherit" && r.Mode != "custom" {
		return common.ErrParam
	}
	if !regexpService.MatchString(r.Service) || len(r.Tags) > 7 {
		return common.ErrParam
	}
	switch r.Kind {
	case "temple":
		n, e := strconv.ParseInt(r.ID, 10, 64)
		if e != nil || n < 1 {
			return common.ErrParam
		}
	case "master":
		if strings.TrimSpace(r.ID) == "" || len(r.ID) > 64 {
			return common.ErrParam
		}
	case "rule":
		if r.Mode != "custom" {
			return common.ErrParam
		}
	default:
		return common.ErrParam
	}
	allowed := map[string]bool{"peace": true, "wealth": true, "love": true, "career": true, "study": true, "taisui": true, "rite": true}
	seen := map[string]bool{}
	for _, t := range r.Tags {
		if !allowed[t] || seen[t] {
			return common.ErrParam
		}
		seen[t] = true
	}
	if r.Mode == "inherit" && len(r.Tags) > 0 {
		return common.ErrParam
	}
	return nil
}

func UpdateServiceIntentions(ctx context.Context, db sqlx.SqlConn, r ServiceIntentionUpdate) error {
	if e := r.Validate(); e != nil {
		return e
	}
	return db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var count int
		// Lock the provider service before replacing its binding set.
		var check string
		var args []interface{}
		switch r.Kind {
		case "master":
			check = "SELECT id FROM askxuan_master.master_service_tag WHERE master_code=? AND service_code=? FOR UPDATE"
			args = []interface{}{r.ID, r.Service}
		case "temple":
			check = "SELECT id FROM askxuan_temple.temple_service WHERE id=? AND service_code=? FOR UPDATE"
			args = []interface{}{r.ID, r.Service}
		case "rule":
			check = "SELECT 1 FROM askxuan_temple.service_type WHERE code=? FOR UPDATE"
			args = []interface{}{r.Service}
		}
		if e := tx.QueryRowCtx(ctx, &count, check, args...); e != nil {
			if e == sqlx.ErrNotFound {
				return common.ErrParam
			}
			return e
		}
		if count == 0 {
			return common.ErrParam
		}
		for _, tag := range r.Tags {
			if e := tx.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM intent_tag WHERE code=? AND status='enabled'", tag); e != nil {
				return e
			}
			if count != 1 {
				return common.ErrParam
			}
		}
		var del, ins string
		var key []interface{}
		switch r.Kind {
		case "master":
			if _, e := tx.ExecCtx(ctx, "UPDATE askxuan_master.master_service_tag SET intent_mode=? WHERE master_code=? AND service_code=?", r.Mode, r.ID, r.Service); e != nil {
				return e
			}
			del = "DELETE FROM master_service_intention WHERE master_code=? AND service_code=?"
			ins = "INSERT INTO master_service_intention(master_code,service_code,tag_code) VALUES(?,?,?)"
			key = []interface{}{r.ID, r.Service}
		case "temple":
			if _, e := tx.ExecCtx(ctx, "UPDATE askxuan_temple.temple_service SET intent_mode=? WHERE id=?", r.Mode, r.ID); e != nil {
				return e
			}
			del = "DELETE FROM askxuan_temple.temple_service_intent_tag WHERE temple_service_id=?"
			ins = "INSERT INTO askxuan_temple.temple_service_intent_tag(temple_service_id,tag_code) VALUES(?,?)"
			key = []interface{}{r.ID}
		case "rule":
			del = "DELETE FROM service_intention_rule WHERE service_code=?"
			ins = "INSERT INTO service_intention_rule(service_code,tag_code) VALUES(?,?)"
			key = []interface{}{r.Service}
		}
		if _, e := tx.ExecCtx(ctx, del, key...); e != nil {
			return e
		}
		for _, tag := range r.Tags {
			values := append(append([]interface{}{}, key...), tag)
			if _, e := tx.ExecCtx(ctx, ins, values...); e != nil {
				return e
			}
		}
		return nil
	})
}

type ServiceIntentionBinding struct {
	Kind        string `db:"kind" json:"kind"`
	ID          string `db:"id" json:"id"`
	Name        string `db:"name" json:"name"`
	Service     string `db:"service_code" json:"serviceCode"`
	ServiceName string `db:"service_name" json:"serviceName"`
	Tags        string `db:"tags" json:"tags"`
	Mode        string `db:"mode" json:"mode"`
}

func ListServiceIntentions(ctx context.Context, db sqlx.SqlConn) ([]ServiceIntentionBinding, error) {
	query := fmt.Sprintf(`SELECT 'master' kind,m.code id,m.dharma_name name,ms.service_code,st.name service_name,%s tags,ms.intent_mode mode FROM askxuan_master.master m JOIN askxuan_master.master_service_tag ms ON ms.master_code=m.code JOIN askxuan_temple.service_type st ON st.code=ms.service_code
 UNION ALL SELECT 'temple',CAST(ts.id AS CHAR),t.name,ts.service_code,ts.service_name,%s,ts.intent_mode FROM askxuan_temple.temple_service ts JOIN askxuan_temple.temple t ON t.code=ts.temple_code
 UNION ALL SELECT 'rule',st.code,'标准服务默认规则',st.code,st.name,%s,'custom' FROM askxuan_temple.service_type st ORDER BY kind,id,service_code`, MasterIntentionTagsSQL("ms"), TempleIntentionTagsSQL("ts"), DefaultIntentionTagsSQL("st.code"))
	rows := []ServiceIntentionBinding{}
	e := db.QueryRowsCtx(ctx, &rows, query)
	return rows, e
}

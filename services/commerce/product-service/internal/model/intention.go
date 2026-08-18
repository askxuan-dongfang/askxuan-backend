package model

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type IntentTag struct {
	Code         string `db:"code"`
	Name         string `db:"name"`
	Description  string `db:"description"`
	Icon         string `db:"icon"`
	LandingType  string `db:"landing_type"`
	LandingValue string `db:"landing_value"`
	ActionTitle  string `db:"action_title"`
	Sort         int    `db:"sort"`
	Status       string `db:"status"`
}

type IntentionResource struct {
	ResourceType string  `db:"resource_type"`
	SourceId     string  `db:"source_id"`
	Title        string  `db:"title"`
	Subtitle     string  `db:"subtitle"`
	Price        float64 `db:"price"`
	Image        string  `db:"image"`
	OrderTarget  string  `db:"order_target"`
	TempleCode   string  `db:"temple_code"`
	ServiceCode  string  `db:"service_code"`
	MasterCode   string  `db:"master_code"`
	UpdatedAt    string  `db:"updated_at"`
}

type IntentionModel interface {
	FindTags(ctx context.Context) ([]*IntentTag, error)
	FindAllTags(ctx context.Context) ([]*IntentTag, error)
	FindTag(ctx context.Context, code string, enabledOnly bool) (*IntentTag, error)
	InsertTag(ctx context.Context, tag *IntentTag) error
	UpdateTag(ctx context.Context, tag *IntentTag) error
	UpdateTagStatus(ctx context.Context, code, status string) error
	FindResources(ctx context.Context, code string, page, size int) ([]*IntentionResource, int64, error)
	FindProductTags(ctx context.Context, productId int64) ([]string, error)
	ReplaceProductTags(ctx context.Context, productId int64, tags []string) error
}

type intentionModel struct{ conn sqlx.SqlConn }

func NewIntentionModel(conn sqlx.SqlConn) IntentionModel { return &intentionModel{conn: conn} }

func IsValidIntentCode(code string) bool {
	return regexp.MustCompile(`^[a-z][a-z0-9_]{1,31}$`).MatchString(code)
}

func ValidateIntentCodes(codes []string) error {
	for _, code := range codes {
		if !IsValidIntentCode(code) {
			return fmt.Errorf("invalid intent code %q", code)
		}
	}
	return nil
}

func (m *intentionModel) FindTags(ctx context.Context) ([]*IntentTag, error) {
	var tags []*IntentTag
	err := m.conn.QueryRowsCtx(ctx, &tags, "SELECT code,name,description,icon,landing_type,landing_value,action_title,sort,status FROM intent_tag WHERE status='enabled' ORDER BY sort,code")
	return tags, err
}

func (m *intentionModel) FindAllTags(ctx context.Context) ([]*IntentTag, error) {
	var tags []*IntentTag
	err := m.conn.QueryRowsCtx(ctx, &tags, "SELECT code,name,description,icon,landing_type,landing_value,action_title,sort,status FROM intent_tag ORDER BY sort,code")
	return tags, err
}

func (m *intentionModel) FindTag(ctx context.Context, code string, enabledOnly bool) (*IntentTag, error) {
	query := "SELECT code,name,description,icon,landing_type,landing_value,action_title,sort,status FROM intent_tag WHERE code=?"
	if enabledOnly {
		query += " AND status='enabled'"
	}
	var tag IntentTag
	err := m.conn.QueryRowCtx(ctx, &tag, query, code)
	return &tag, err
}

func (m *intentionModel) InsertTag(ctx context.Context, tag *IntentTag) error {
	_, err := m.conn.ExecCtx(ctx, "INSERT INTO intent_tag(code,name,description,icon,landing_type,landing_value,action_title,sort,status) VALUES(?,?,?,?,?,?,?,?,?)", tag.Code, tag.Name, tag.Description, tag.Icon, tag.LandingType, tag.LandingValue, tag.ActionTitle, tag.Sort, tag.Status)
	return err
}

func (m *intentionModel) UpdateTag(ctx context.Context, tag *IntentTag) error {
	_, err := m.conn.ExecCtx(ctx, "UPDATE intent_tag SET name=?,description=?,icon=?,landing_type=?,landing_value=?,action_title=?,sort=? WHERE code=?", tag.Name, tag.Description, tag.Icon, tag.LandingType, tag.LandingValue, tag.ActionTitle, tag.Sort, tag.Code)
	return err
}

func (m *intentionModel) UpdateTagStatus(ctx context.Context, code, status string) error {
	_, err := m.conn.ExecCtx(ctx, "UPDATE intent_tag SET status=? WHERE code=?", status, code)
	return err
}

func resourceUnion(code string) (string, []interface{}) {
	// 按心愿办：聚合寺院服务 + 双轨大师服务（寺庙绑定+野生，不含商品）
	serviceWhere := "ts.status='on_shelf' AND t.status='正常'"
	// 双轨大师：已认证+上架+正常，服务标签 enabled（两类大师都可直约/经寺院服务下单）
	masterWhere := "mst.status='enabled' AND m.auth_status='已认证' AND m.shelf_status='on_shelf' AND m.platform_status='normal'"
	var args []interface{}
	if code != "" {
		serviceWhere += " AND tsit.tag_code=?"
		// 大师分支：心愿对应的标准服务编码（service 型诉求的 landing_value）
		masterWhere += " AND mst.service_code IN (SELECT landing_value FROM intent_tag WHERE code=? AND landing_type='service')"
		args = append(args, code, code)
	}
	query := fmt.Sprintf(`
SELECT DISTINCT 'service' resource_type, CAST(ts.id AS CHAR) source_id,
  CONCAT(t.name,' · ',ts.service_name) title, COALESCE(t.description,'') subtitle,
  ts.price, t.cover_image image,
  CONCAT('service:',ts.temple_code,':',ts.service_code) order_target,
  ts.temple_code, ts.service_code, '' master_code,
  DATE_FORMAT(ts.update_time,'%%Y-%%m-%%d %%H:%%i:%%s') updated_at
FROM askxuan_temple.temple_service ts
JOIN askxuan_temple.temple t ON t.code=ts.temple_code
JOIN askxuan_temple.temple_service_intent_tag tsit ON tsit.temple_service_id=ts.id
WHERE %s
UNION ALL
SELECT DISTINCT 'master' resource_type, mst.master_code source_id,
  CONCAT(m.dharma_name,' · ',st.name) title, CONCAT(m.type,' · ',m.sect) subtitle,
  mst.price, COALESCE(m.avatar,'') image,
  CONCAT('master:',mst.master_code,':',mst.service_code) order_target,
  m.temple_code temple_code, mst.service_code service_code, mst.master_code master_code,
  DATE_FORMAT(mst.update_time,'%%Y-%%m-%%d %%H:%%i:%%s') updated_at
FROM askxuan_master.master_service_tag mst
JOIN askxuan_master.master m ON m.code=mst.master_code
JOIN askxuan_temple.service_type st ON st.code=mst.service_code
WHERE %s`, serviceWhere, masterWhere)
	return query, args
}

func (m *intentionModel) FindResources(ctx context.Context, code string, page, size int) ([]*IntentionResource, int64, error) {
	union, args := resourceUnion(code)
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, "SELECT COUNT(1) FROM ("+union+") intention_resources", args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*IntentionResource{}, 0, nil
	}
	listArgs := append(append([]interface{}{}, args...), (page-1)*size, size)
	var resources []*IntentionResource
	query := "SELECT resource_type,source_id,title,subtitle,price,image,order_target,temple_code,service_code,master_code,updated_at FROM (" + union + ") intention_resources ORDER BY updated_at DESC,resource_type,source_id DESC LIMIT ?,?"
	if err := m.conn.QueryRowsCtx(ctx, &resources, query, listArgs...); err != nil {
		return nil, 0, err
	}
	return resources, total, nil
}

func (m *intentionModel) FindProductTags(ctx context.Context, productId int64) ([]string, error) {
	var rows []struct {
		Code string `db:"tag_code"`
	}
	if err := m.conn.QueryRowsCtx(ctx, &rows, "SELECT tag_code FROM product_intent_tag WHERE product_id=? ORDER BY tag_code", productId); err != nil {
		return nil, err
	}
	tags := make([]string, 0, len(rows))
	for _, row := range rows {
		tags = append(tags, row.Code)
	}
	return tags, nil
}

func (m *intentionModel) ReplaceProductTags(ctx context.Context, productId int64, tags []string) error {
	if err := ValidateIntentCodes(tags); err != nil {
		return err
	}
	return m.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		if _, err := session.ExecCtx(ctx, "DELETE FROM product_intent_tag WHERE product_id=?", productId); err != nil {
			return err
		}
		for _, code := range uniqueCodes(tags) {
			if _, err := session.ExecCtx(ctx, "INSERT INTO product_intent_tag(product_id,tag_code) VALUES(?,?)", productId, code); err != nil {
				return err
			}
		}
		return nil
	})
}

func uniqueCodes(codes []string) []string {
	seen := make(map[string]struct{}, len(codes))
	out := make([]string, 0, len(codes))
	for _, raw := range codes {
		code := strings.TrimSpace(raw)
		if code == "" {
			continue
		}
		if _, ok := seen[code]; ok {
			continue
		}
		seen[code] = struct{}{}
		out = append(out, code)
	}
	return out
}

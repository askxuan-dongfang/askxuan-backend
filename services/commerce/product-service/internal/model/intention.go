package model

import (
	"context"
	"fmt"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var validIntentCodes = map[string]struct{}{
	"peace": {}, "wealth": {}, "love": {}, "career": {},
	"study": {}, "taisui": {}, "diy": {}, "rite": {},
}

type IntentTag struct {
	Code        string `db:"code"`
	Name        string `db:"name"`
	Description string `db:"description"`
	Icon        string `db:"icon"`
	Sort        int    `db:"sort"`
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
	UpdatedAt    string  `db:"updated_at"`
}

type IntentionModel interface {
	FindTags(ctx context.Context) ([]*IntentTag, error)
	FindResources(ctx context.Context, code string, page, size int) ([]*IntentionResource, int64, error)
	FindProductTags(ctx context.Context, productId int64) ([]string, error)
	ReplaceProductTags(ctx context.Context, productId int64, tags []string) error
}

type intentionModel struct{ conn sqlx.SqlConn }

func NewIntentionModel(conn sqlx.SqlConn) IntentionModel { return &intentionModel{conn: conn} }

func IsValidIntentCode(code string) bool {
	_, ok := validIntentCodes[code]
	return ok
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
	err := m.conn.QueryRowsCtx(ctx, &tags, "SELECT code,name,description,icon,sort FROM intent_tag WHERE status='enabled' ORDER BY sort,code")
	return tags, err
}

func resourceUnion(code string) (string, []interface{}) {
	productWhere := "p.status='on_shelf' AND p.stock > 0"
	serviceWhere := "ts.status='on_shelf' AND t.status='正常'"
	var args []interface{}
	if code != "" {
		productWhere += " AND pit.tag_code=?"
		serviceWhere += " AND tsit.tag_code=?"
		args = append(args, code, code)
	}
	query := fmt.Sprintf(`
SELECT DISTINCT 'product' resource_type, CAST(p.id AS CHAR) source_id, p.name title,
  COALESCE(p.description,'') subtitle, p.price, p.main_image image,
  CONCAT('product:',p.id) order_target, '' temple_code, '' service_code,
  DATE_FORMAT(p.update_time,'%%Y-%%m-%%d %%H:%%i:%%s') updated_at
FROM product p JOIN product_intent_tag pit ON pit.product_id=p.id WHERE %s
UNION ALL
SELECT DISTINCT 'service' resource_type, CAST(ts.id AS CHAR) source_id,
  CONCAT(t.name,' · ',ts.service_name) title, COALESCE(t.description,'') subtitle,
  ts.price, t.cover_image image,
  CONCAT('service:',ts.temple_code,':',ts.service_code) order_target,
  ts.temple_code, ts.service_code,
  DATE_FORMAT(ts.update_time,'%%Y-%%m-%%d %%H:%%i:%%s') updated_at
FROM askxuan_temple.temple_service ts
JOIN askxuan_temple.temple t ON t.code=ts.temple_code
JOIN askxuan_temple.temple_service_intent_tag tsit ON tsit.temple_service_id=ts.id
WHERE %s`, productWhere, serviceWhere)
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
	query := "SELECT resource_type,source_id,title,subtitle,price,image,order_target,temple_code,service_code,updated_at FROM (" + union + ") intention_resources ORDER BY updated_at DESC,resource_type,source_id DESC LIMIT ?,?"
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

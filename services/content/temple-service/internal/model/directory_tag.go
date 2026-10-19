package model

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/askxuan/common"
	"github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type DirectoryTag struct {
	Code       string `db:"code" json:"code"`
	Group      string `db:"tag_group" json:"group"`
	BeliefCode string `db:"belief_code" json:"beliefCode,optional"`
	Name       string `db:"name" json:"name"`
	Value      string `db:"value" json:"value"`
	Sort       int    `db:"sort" json:"sort"`
	Status     string `db:"status" json:"status"`
}
type DirectoryTagModel struct{ conn sqlx.SqlConn }

func NewDirectoryTagModel(conn sqlx.SqlConn) *DirectoryTagModel {
	return &DirectoryTagModel{conn: conn}
}
func ValidateDirectoryTag(p *DirectoryTag) error {
	p.Name = strings.TrimSpace(p.Name)
	p.Value = strings.TrimSpace(p.Value)
	p.BeliefCode = strings.TrimSpace(p.BeliefCode)
	if p.BeliefCode == "taoism" {
		p.BeliefCode = "daoism"
	}
	if !IsValidBeliefCode(p.Code) || (p.Group != "tradition" && p.Group != "position" && p.Group != "specialty") || p.Name == "" || p.Value == "" || utf8.RuneCountInString(p.Name) > 32 || utf8.RuneCountInString(p.Value) > 64 || strings.Contains(p.Value, ",") || p.Sort < 0 || p.Sort > 9999 || (p.Status != "enabled" && p.Status != "disabled") {
		return common.NewBizError(40010, "请填写有效的标签分组、编码、名称、匹配值、排序和状态")
	}
	if p.Group == "tradition" {
		switch p.BeliefCode {
		case "han_buddhism", "tibetan_buddhism", "theravada_buddhism", "daoism", "folk":
		default:
			return common.NewBizError(40010, "请选择宗派或信俗所属的信仰类型")
		}
	} else if p.BeliefCode != "" {
		return common.NewBizError(40010, "仅宗派或信俗标签可设置所属信仰类型")
	}
	return nil
}
func (m *DirectoryTagModel) List(ctx context.Context, admin bool) ([]DirectoryTag, error) {
	rows := []DirectoryTag{}
	q := "SELECT code,tag_group,belief_code,name,value,sort,status FROM askxuan_temple.directory_tag"
	if !admin {
		q += " WHERE status='enabled'"
	}
	err := m.conn.QueryRowsCtx(ctx, &rows, q+" ORDER BY tag_group,sort,code")
	return rows, err
}
func (m *DirectoryTagModel) Save(ctx context.Context, p *DirectoryTag, create bool) error {
	var err error
	if create {
		if err := ValidateDirectoryTag(p); err != nil {
			return err
		}
		_, err = m.conn.ExecCtx(ctx, "INSERT INTO askxuan_temple.directory_tag(code,tag_group,belief_code,name,value,sort,status) VALUES(?,?,?,?,?,?,?)", p.Code, p.Group, p.BeliefCode, p.Name, p.Value, p.Sort, p.Status)
	} else {
		var existing DirectoryTag
		if err = m.conn.QueryRowCtx(ctx, &existing, "SELECT code,tag_group,belief_code,name,value,sort,status FROM askxuan_temple.directory_tag WHERE code=?", p.Code); errors.Is(err, sqlx.ErrNotFound) {
			return common.NewBizError(40410, "标签不存在")
		}
		if err != nil {
			return err
		}
		if p.Group != existing.Group || p.Value != existing.Value {
			return common.NewBizError(40010, "标签分组与匹配值创建后不可更改")
		}
		// Older admin clients omit the new field; renaming must not erase its parent.
		preserveParent := p.BeliefCode == ""
		if preserveParent {
			p.BeliefCode = existing.BeliefCode
		}
		if err := ValidateDirectoryTag(p); err != nil {
			return err
		}
		_, err = m.conn.ExecCtx(ctx, "UPDATE askxuan_temple.directory_tag SET name=?,sort=?,status=?,belief_code=IF(?,belief_code,?) WHERE code=?", p.Name, p.Sort, p.Status, preserveParent, p.BeliefCode, p.Code)
	}
	var duplicate *mysql.MySQLError
	if errors.As(err, &duplicate) && duplicate.Number == 1062 {
		return common.NewBizError(40910, "标签编码或同组匹配值已存在")
	}
	return err
}
func providerTable(kind string) (string, error) {
	if kind == "temple" {
		return "askxuan_temple.temple", nil
	}
	if kind == "master" {
		return "askxuan_master.master", nil
	}
	return "", common.ErrParamInvalid
}
func (m *DirectoryTagModel) Assigned(ctx context.Context, kind, id string) ([]string, error) {
	if _, err := providerTable(kind); err != nil {
		return nil, err
	}
	rows := []string{}
	err := m.conn.QueryRowsCtx(ctx, &rows, "SELECT tag_code FROM askxuan_temple.provider_directory_tag WHERE provider_kind=? AND provider_code=? ORDER BY tag_code", kind, id)
	return rows, err
}
func (m *DirectoryTagModel) Assign(ctx context.Context, kind, id string, codes []string) error {
	table, err := providerTable(kind)
	if err != nil {
		return err
	}
	if id == "" || len(id) > 64 || len(codes) > 12 {
		return common.ErrParamInvalid
	}
	seen := map[string]bool{}
	for _, code := range codes {
		if !IsValidBeliefCode(code) || seen[code] {
			return common.ErrParamInvalid
		}
		seen[code] = true
	}
	return m.conn.TransactCtx(ctx, func(ctx context.Context, s sqlx.Session) error {
		var count int
		if err := s.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM "+table+" WHERE code=?", id); err != nil {
			return err
		}
		if count == 0 {
			return common.NewBizError(40410, "寺院或师傅不存在")
		}
		// Validate every tag before replacing; a disabled/unknown tag cannot erase old assignments.
		for _, code := range codes {
			var tag DirectoryTag
			err := s.QueryRowCtx(ctx, &tag, "SELECT code,tag_group,belief_code,name,value,sort,status FROM askxuan_temple.directory_tag WHERE code=? FOR UPDATE", code)
			if errors.Is(err, sqlx.ErrNotFound) {
				return common.NewBizError(40010, "请选择已启用的宗派或信俗标签")
			}
			if err != nil {
				return err
			}
			if tag.Group != "tradition" || tag.Status != "enabled" {
				return common.NewBizError(40010, "请选择已启用的宗派或信俗标签")
			}
		}
		if _, err := s.ExecCtx(ctx, "DELETE FROM askxuan_temple.provider_directory_tag WHERE provider_kind=? AND provider_code=?", kind, id); err != nil {
			return err
		}
		for _, code := range codes {
			if _, err := s.ExecCtx(ctx, "INSERT INTO askxuan_temple.provider_directory_tag(provider_kind,provider_code,tag_code) VALUES(?,?,?)", kind, id, code); err != nil {
				return err
			}
		}
		return nil
	})
}

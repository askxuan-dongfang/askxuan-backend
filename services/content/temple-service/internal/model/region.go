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

type RegionTag struct {
	Code   string `db:"code" json:"code"`
	Name   string `db:"name" json:"name"`
	Sort   int    `db:"sort" json:"sort"`
	Status string `db:"status" json:"status"`
}

type RegionModel struct{ conn sqlx.SqlConn }

func NewRegionModel(conn sqlx.SqlConn) *RegionModel { return &RegionModel{conn: conn} }

func ValidateRegion(p *RegionTag) error {
	p.Name = strings.TrimSpace(p.Name)
	if !IsValidBeliefCode(p.Code) || p.Name == "" || utf8.RuneCountInString(p.Name) > 32 || p.Sort < 0 || p.Sort > 9999 || (p.Status != "enabled" && p.Status != "disabled") {
		return common.NewBizError(40010, "请填写有效的地区编码、名称、排序和状态")
	}
	return nil
}

func (m *RegionModel) List(ctx context.Context, includeDisabled bool) ([]RegionTag, error) {
	query := "SELECT code,name,sort,status FROM askxuan_temple.region_tag"
	if !includeDisabled {
		query += " WHERE status='enabled'"
	}
	list := []RegionTag{}
	err := m.conn.QueryRowsCtx(ctx, &list, query+" ORDER BY sort,code")
	return list, err
}

func (m *RegionModel) Save(ctx context.Context, p *RegionTag, create bool) error {
	if err := ValidateRegion(p); err != nil {
		return err
	}
	var err error
	if create {
		_, err = m.conn.ExecCtx(ctx, "INSERT INTO askxuan_temple.region_tag(code,name,sort,status) VALUES(?,?,?,?)", p.Code, p.Name, p.Sort, p.Status)
	} else {
		var count int
		if err = m.conn.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM askxuan_temple.region_tag WHERE code=?", p.Code); err != nil {
			return err
		}
		if count == 0 {
			return common.NewBizError(40410, "地区标签不存在")
		}
		_, err = m.conn.ExecCtx(ctx, "UPDATE askxuan_temple.region_tag SET name=?,sort=?,status=? WHERE code=?", p.Name, p.Sort, p.Status, p.Code)
	}
	var duplicate *mysql.MySQLError
	if errors.As(err, &duplicate) && duplicate.Number == 1062 {
		return common.NewBizError(40910, "地区编码或名称已存在")
	}
	return err
}

// Empty assignment removes the override. Temple-bound masters then inherit the temple region.
func (m *RegionModel) Assign(ctx context.Context, kind, id, code string) error {
	table := "askxuan_temple.temple"
	if kind == "master" {
		table = "askxuan_master.master"
	} else if kind != "temple" {
		return common.ErrParamInvalid
	}
	if id == "" || len(id) > 64 || (code != "" && !IsValidBeliefCode(code)) {
		return common.ErrParamInvalid
	}
	return m.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		var count int
		if err := session.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM "+table+" WHERE code=?", id); err != nil {
			return err
		}
		if count == 0 {
			return common.NewBizError(40410, "寺院或师傅不存在")
		}
		if code == "" {
			_, err := session.ExecCtx(ctx, "DELETE FROM askxuan_temple.provider_region WHERE provider_kind=? AND provider_code=?", kind, id)
			return err
		}
		var status string
		err := session.QueryRowCtx(ctx, &status, "SELECT status FROM askxuan_temple.region_tag WHERE code=? FOR UPDATE", code)
		if errors.Is(err, sqlx.ErrNotFound) {
			return common.NewBizError(40010, "请选择已启用的地区标签")
		}
		if err != nil {
			return err
		}
		if status != "enabled" {
			return common.NewBizError(40010, "请选择已启用的地区标签")
		}
		_, err = session.ExecCtx(ctx, "INSERT INTO askxuan_temple.provider_region(provider_kind,provider_code,region_code) VALUES(?,?,?) ON DUPLICATE KEY UPDATE region_code=VALUES(region_code)", kind, id, code)
		return err
	})
}

func templeRegionCode(codeColumn string) string {
	return "COALESCE((SELECT pr.region_code FROM askxuan_temple.provider_region pr WHERE pr.provider_kind='temple' AND pr.provider_code=" + codeColumn + "),'')"
}
func templeRegionColumns(codeColumn string) string {
	code := templeRegionCode(codeColumn)
	return code + " AS region_code, COALESCE((SELECT rt.name FROM askxuan_temple.region_tag rt WHERE rt.code=" + code + "),'') AS region_name"
}

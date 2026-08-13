package model

import (
	"context"
	"fmt"
)

const (
	BizTypeBooking      = "booking"
	BizTypeConsultation = "consultation"
	BizTypeDiyBlessing  = "diy_blessing"
	BizTypeDiyMaterial  = "diy_material"
	BizTypeShopOrder    = "shop_order"
)

type CommissionConfig struct {
	Id          int64   `db:"id" json:"id"`
	BizType     string  `db:"biz_type" json:"bizType"`
	Rate        float64 `db:"rate" json:"rate"`
	Description string  `db:"description" json:"description"`
	UpdateTime  string  `db:"update_time" json:"updateTime"`
}

func ListCommissionConfigs(bizType string) []CommissionConfig {
	query := `SELECT id,biz_type,rate,description,DATE_FORMAT(update_time,'%Y-%m-%d %H:%i:%s') update_time FROM commission_config`
	args := []interface{}{}
	if bizType != "" {
		query += " WHERE biz_type=?"
		args = append(args, bizType)
	}
	query += " ORDER BY id"
	var list []CommissionConfig
	if db.QueryRowsCtx(context.Background(), &list, query, args...) != nil {
		return []CommissionConfig{}
	}
	return list
}
func FindCommissionConfigByID(id int64) (CommissionConfig, bool) {
	var c CommissionConfig
	if db.QueryRowCtx(context.Background(), &c, `SELECT id,biz_type,rate,description,DATE_FORMAT(update_time,'%Y-%m-%d %H:%i:%s') update_time FROM commission_config WHERE id=?`, id) != nil {
		return CommissionConfig{}, false
	}
	return c, true
}
func UpdateCommissionConfig(id int64, rate float64, description string) error {
	res, err := db.ExecCtx(context.Background(), `UPDATE commission_config SET rate=?,description=COALESCE(NULLIF(?,''),description) WHERE id=?`, rate, description, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("配置不存在")
	}
	return nil
}

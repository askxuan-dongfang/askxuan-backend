package logic

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"

	"github.com/askxuan/common"
	"github.com/askxuan/common/rpc/catalog"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type pieceRate struct {
	First    int64    `json:"first"`
	FirstFee *float64 `json:"firstFee"`
	Next     int64    `json:"next"`
	NextFee  *float64 `json:"nextFee"`
}

func calculateFreight(ctx context.Context, db sqlx.SqlConn, userID, addressID int64, items []*catalog.CatalogQuote) (int64, error) {
	quantities := make(map[int64]int64)
	for _, item := range items {
		if item.FreightTemplateId > 0 {
			quantities[item.FreightTemplateId] += int64(item.Quantity)
		}
	}
	if len(quantities) == 0 {
		return 0, nil
	}

	var province string
	err := db.QueryRowCtx(ctx, &province, `SELECT province FROM askxuan_user.user_address WHERE id=? AND user_id=?`, addressID, userID)
	if errors.Is(err, sqlx.ErrNotFound) || (err == nil && strings.TrimSpace(province) == "") {
		return 0, common.NewBizError(common.ErrParamInvalid.Code, "请选择有省份的有效收货地址")
	}
	if err != nil {
		return 0, err
	}

	var total int64
	for id, quantity := range quantities {
		var template struct {
			Type string `db:"type"`
			Free bool   `db:"free_shipping"`
			JSON string `db:"config"`
		}
		err := db.QueryRowCtx(ctx, &template, `SELECT type,free_shipping,COALESCE(config,'') config FROM askxuan_logistics.freight_template WHERE id=? AND status='enabled'`, id)
		if errors.Is(err, sqlx.ErrNotFound) {
			return 0, common.NewBizError(common.ErrOrderStatusConflict.Code, "部分商品的运费模板未启用")
		}
		if err != nil {
			return 0, err
		}
		if template.Free {
			continue
		}
		if template.Type != "by_piece" {
			return 0, common.NewBizError(common.ErrOrderStatusConflict.Code, "部分商品暂未配置可用的按件运费")
		}
		cents, ok := pieceFreight(template.JSON, province, quantity)
		if !ok || total+cents > 9999999999 {
			return 0, common.NewBizError(common.ErrOrderStatusConflict.Code, "部分商品的运费规则无法计算")
		}
		total += cents
	}
	return total, nil
}

func pieceFreight(config, province string, quantity int64) (int64, bool) {
	var rule struct {
		pieceRate
		Default *pieceRate           `json:"default"`
		Regions map[string]pieceRate `json:"regions"`
	}
	if err := json.Unmarshal([]byte(config), &rule); err != nil {
		return 0, false
	}
	rate := rule.pieceRate
	if rule.Default != nil {
		rate = *rule.Default
	}
	province = strings.TrimSpace(province)
	if regional, ok := rule.Regions[province]; ok {
		rate = regional
	} else {
		// Address forms also accept common short names, such as 广东 and 上海.
		short := strings.TrimRight(province, "省市")
		for _, name := range []string{short, short + "省", short + "市"} {
			if regional, ok := rule.Regions[name]; ok {
				rate = regional
				break
			}
		}
	}
	if rate.First < 1 || rate.Next < 1 || rate.FirstFee == nil || rate.NextFee == nil ||
		*rate.FirstFee < 0 || *rate.FirstFee > 99999999.99 || *rate.NextFee < 0 || *rate.NextFee > 99999999.99 {
		return 0, false
	}
	cents := int64(math.Round(*rate.FirstFee * 100))
	if extra := quantity - rate.First; extra > 0 {
		steps := 1 + (extra-1)/rate.Next
		next := int64(math.Round(*rate.NextFee * 100))
		if next > 0 && steps > (9999999999-cents)/next {
			return 0, false
		}
		cents += steps * next
	}
	return cents, true
}

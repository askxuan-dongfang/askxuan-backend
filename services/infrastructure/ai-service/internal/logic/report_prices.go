package logic

import (
	"context"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const DefaultReportPriceCents int64 = 990
const MaxReportPriceCents int64 = 100000000 // Same integer-cent ceiling as the cash wallet.

var ErrReportPriceConflict = common.NewBizError(40931, "专题价格已被更新，请刷新后重试")

type ReportPriceProduct struct {
	Code       string `db:"code" json:"code"`
	Title      string `db:"title" json:"title"`
	Subtitle   string `db:"subtitle" json:"subtitle"`
	PriceCents int64  `db:"price_cents" json:"priceCents"`
	Revision   int64  `db:"price_revision" json:"revision"`
	Enabled    bool   `db:"enabled" json:"enabled"`
	Version    string `db:"version" json:"version"`
}
type ReportPriceUpdate struct {
	PriceCents int64 `json:"priceCents"`
	Revision   int64 `json:"revision"`
}

const reportPriceColumns = "code,title,subtitle,price_cents,price_revision,enabled,version"

func ReportPriceProducts(ctx context.Context, db sqlx.SqlConn) ([]ReportPriceProduct, error) {
	rows := []ReportPriceProduct{}
	err := db.QueryRowsCtx(ctx, &rows, "SELECT "+reportPriceColumns+" FROM ai_report_product ORDER BY code")
	return rows, err
}

func UpdateReportPrice(ctx context.Context, db sqlx.SqlConn, actor, code string, req ReportPriceUpdate) (ReportPriceProduct, error) {
	var row ReportPriceProduct
	if actor == "" || len(code) == 0 || len(code) > 32 || req.PriceCents < 1 || req.PriceCents > MaxReportPriceCents || req.Revision < 1 {
		return row, common.ErrParam
	}
	err := db.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		if err := tx.QueryRowCtx(ctx, &row, "SELECT "+reportPriceColumns+" FROM ai_report_product WHERE code=? FOR UPDATE", code); err != nil {
			if err == sqlx.ErrNotFound {
				return common.ErrParam
			}
			return err
		}
		if row.Revision != req.Revision {
			return ErrReportPriceConflict
		}
		if row.PriceCents == req.PriceCents {
			return nil
		}
		if _, err := tx.ExecCtx(ctx, "INSERT INTO ai_report_price_audit(product_code,actor_id,old_price_cents,new_price_cents,revision) VALUES(?,?,?,?,?)", code, actor, row.PriceCents, req.PriceCents, row.Revision+1); err != nil {
			return err
		}
		if _, err := tx.ExecCtx(ctx, "UPDATE ai_report_product SET price_cents=?,price_revision=price_revision+1 WHERE code=? AND price_revision=?", req.PriceCents, code, row.Revision); err != nil {
			return err
		}
		row.PriceCents = req.PriceCents
		row.Revision++
		return nil
	})
	return row, err
}

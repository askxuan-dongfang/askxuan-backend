package model

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const shopOrderLogisticsTable = "askxuan_shop.shop_order_logistics"

// ShopOrderLogistics 订单物流表
type ShopOrderLogistics struct {
	Id             int64  `db:"id" json:"id"`
	OrderId        int64  `db:"order_id" json:"orderId"`
	ExpressCompany string `db:"express_company" json:"expressCompany"`
	TrackingNo     string `db:"tracking_no" json:"trackingNo"`
	ShipTime       string `db:"ship_time" json:"shipTime"`
}

// ShopOrderLogisticsModel 物流模型接口
type ShopOrderLogisticsModel interface {
	Insert(ctx context.Context, data *ShopOrderLogistics) (*ShopOrderLogistics, error)
	FindByOrderId(ctx context.Context, orderId int64) (*ShopOrderLogistics, error)
}

type defaultShopOrderLogisticsModel struct {
	conn sqlx.SqlConn
}

func NewShopOrderLogisticsModel(conn sqlx.SqlConn) ShopOrderLogisticsModel {
	return &defaultShopOrderLogisticsModel{conn: conn}
}

func (m *defaultShopOrderLogisticsModel) Insert(ctx context.Context, data *ShopOrderLogistics) (*ShopOrderLogistics, error) {
	if data.ShipTime == "" {
		data.ShipTime = time.Now().Format("2006-01-02 15:04:05")
	}
	query := fmt.Sprintf(`INSERT INTO %s (order_id, express_company, tracking_no, ship_time) VALUES (?, ?, ?, ?)`, shopOrderLogisticsTable)
	result, err := m.conn.ExecCtx(ctx, query, data.OrderId, data.ExpressCompany, data.TrackingNo, data.ShipTime)
	if err != nil {
		return nil, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}
	data.Id = id
	return data, nil
}

func (m *defaultShopOrderLogisticsModel) FindByOrderId(ctx context.Context, orderId int64) (*ShopOrderLogistics, error) {
	var l ShopOrderLogistics
	query := fmt.Sprintf(`SELECT id, order_id, express_company, tracking_no, ship_time FROM %s WHERE order_id = ?`, shopOrderLogisticsTable)
	err := m.conn.QueryRowCtx(ctx, &l, query, orderId)
	if err != nil {
		return nil, err
	}
	return &l, nil
}

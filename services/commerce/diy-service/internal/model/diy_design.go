package model

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// 设计状态常量
const (
	DesignStatusPrivate       = "private"        // 私密
	DesignStatusPublic        = "public"         // 公开
	DesignStatusPendingReview = "pending_review" // 待审核
	DesignStatusApproved      = "approved"       // 审核通过
	DesignStatusRejected      = "rejected"       // 审核驳回
)

const diyDesignTable = "askxuan_diy.diy_design"

// DiyDesign DIY设计表
type DiyDesign struct {
	Id               int64   `db:"id" json:"id"`
	DesignNo         string  `db:"design_no" json:"designNo"`
	UserId           string  `db:"user_id" json:"userId"`
	Name             string  `db:"name" json:"name"`
	DesignData       string  `db:"design_data" json:"designData"`
	TotalPrice       float64 `db:"total_price" json:"totalPrice"`
	Status           string  `db:"status" json:"status"`
	BlessServiceCode string  `db:"bless_service_code" json:"blessServiceCode"`
	CreateTime       string  `db:"create_time" json:"createTime"`
	UpdateTime       string  `db:"update_time" json:"updateTime"`
}

// DiyDesignWithOrder 我的设计（含最新订单信息，无订单时 OrderNo/OrderStatus 为空）
type DiyDesignWithOrder struct {
	DiyDesign
	OrderNo     string `db:"order_no" json:"orderNo"`
	OrderStatus string `db:"order_status" json:"orderStatus"`
}

// DiyDesignModel 设计模型接口
type DiyDesignModel interface {
	Insert(ctx context.Context, data *DiyDesign) (*DiyDesign, error)
	FindOne(ctx context.Context, id int64) (*DiyDesign, error)
	FindListPublic(ctx context.Context, page, size int) ([]*DiyDesign, int64, error)
	FindListByUserWithOrders(ctx context.Context, userId string, page, size int) ([]*DiyDesignWithOrder, int64, error)
	Update(ctx context.Context, data *DiyDesign) error
}

type defaultDiyDesignModel struct {
	conn sqlx.SqlConn
}

func NewDiyDesignModel(conn sqlx.SqlConn) DiyDesignModel {
	return &defaultDiyDesignModel{conn: conn}
}

func (m *defaultDiyDesignModel) Insert(ctx context.Context, data *DiyDesign) (*DiyDesign, error) {
	if data.DesignNo == "" {
		data.DesignNo = "D" + time.Now().Format("20060102") + fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	}
	if data.Status == "" {
		data.Status = DesignStatusPrivate
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	data.CreateTime = now
	data.UpdateTime = now

	query := fmt.Sprintf(`INSERT INTO %s (design_no, user_id, name, design_data, total_price, status, bless_service_code, create_time, update_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, diyDesignTable)
	result, err := m.conn.ExecCtx(ctx, query, data.DesignNo, data.UserId, data.Name, data.DesignData, data.TotalPrice, data.Status, data.BlessServiceCode, data.CreateTime, data.UpdateTime)
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

func (m *defaultDiyDesignModel) FindOne(ctx context.Context, id int64) (*DiyDesign, error) {
	var d DiyDesign
	query := fmt.Sprintf(`SELECT id, design_no, user_id, name, design_data, total_price, status, bless_service_code, create_time, update_time FROM %s WHERE id = ?`, diyDesignTable)
	err := m.conn.QueryRowCtx(ctx, &d, query, id)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func (m *defaultDiyDesignModel) FindListPublic(ctx context.Context, page, size int) ([]*DiyDesign, int64, error) {
	countQuery := fmt.Sprintf(`SELECT COUNT(1) FROM %s WHERE status = ?`, diyDesignTable)
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, countQuery, DesignStatusPublic); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*DiyDesign{}, 0, nil
	}

	offset := (page - 1) * size
	listQuery := fmt.Sprintf(`SELECT id, design_no, user_id, name, design_data, total_price, status, bless_service_code, create_time, update_time FROM %s WHERE status = ? ORDER BY create_time DESC LIMIT ?, ?`, diyDesignTable)
	var list []*DiyDesign
	if err := m.conn.QueryRowsCtx(ctx, &list, listQuery, DesignStatusPublic, offset, size); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *defaultDiyDesignModel) FindListByUserWithOrders(ctx context.Context, userId string, page, size int) ([]*DiyDesignWithOrder, int64, error) {
	countQuery := fmt.Sprintf(`SELECT COUNT(1) FROM %s WHERE user_id = ?`, diyDesignTable)
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, countQuery, userId); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*DiyDesignWithOrder{}, 0, nil
	}

	offset := (page - 1) * size
	listQuery := fmt.Sprintf(`SELECT d.id, d.design_no, d.user_id, d.name, d.design_data, d.total_price, d.status, d.bless_service_code, d.create_time, d.update_time,
		COALESCE(o.order_no,'') AS order_no, COALESCE(o.status,'') AS order_status
		FROM %s d
		LEFT JOIN askxuan_diy.diy_order o ON o.design_id = d.id
			AND o.id = (SELECT MAX(id) FROM askxuan_diy.diy_order o2 WHERE o2.design_id = d.id)
		WHERE d.user_id = ? ORDER BY d.update_time DESC LIMIT ?, ?`, diyDesignTable)
	var list []*DiyDesignWithOrder
	if err := m.conn.QueryRowsCtx(ctx, &list, listQuery, userId, offset, size); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *defaultDiyDesignModel) Update(ctx context.Context, data *DiyDesign) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	query := fmt.Sprintf(`UPDATE %s SET name=?, design_data=?, total_price=?, status=?, bless_service_code=?, update_time=? WHERE id=?`, diyDesignTable)
	_, err := m.conn.ExecCtx(ctx, query, data.Name, data.DesignData, data.TotalPrice, data.Status, data.BlessServiceCode, now, data.Id)
	return err
}

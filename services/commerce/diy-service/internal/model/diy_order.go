package model

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// DIY订单状态常量（参照 state-machines.md DIY 手串订单状态机）
const (
	DiyStatusPendingReview      = "pending_review"       // 待审核
	DiyStatusInMaking           = "in_making"            // 制作中
	DiyStatusAwaitingBlessing   = "awaiting_blessing"    // 等待加持
	DiyStatusBlessingInProgress = "blessing_in_progress" // 加持中
	DiyStatusBlessingCompleted  = "blessing_completed"   // 加持完成
	DiyStatusAwaitingShipment   = "awaiting_shipment"    // 待发货
	DiyStatusShipped            = "shipped"              // 已发货
	DiyStatusCompleted          = "completed"            // 已完成
	DiyStatusCancelled          = "cancelled"            // 已取消
	DiyStatusInReturn           = "in_return"            // 售后中
)

// diyValidTransitions DIY订单合法状态流转（参照 state-machines.md 2.3）
var diyValidTransitions = map[string]map[string]bool{
	DiyStatusPendingReview: {
		DiyStatusInMaking:  true,
		DiyStatusCancelled: true,
	},
	DiyStatusInMaking: {
		DiyStatusAwaitingBlessing: true,
		DiyStatusAwaitingShipment: true,
		DiyStatusCancelled:        true,
	},
	DiyStatusAwaitingBlessing: {
		DiyStatusBlessingInProgress: true,
	},
	DiyStatusBlessingInProgress: {
		DiyStatusBlessingCompleted: true,
	},
	DiyStatusBlessingCompleted: {
		DiyStatusAwaitingShipment: true,
	},
	DiyStatusAwaitingShipment: {
		DiyStatusShipped: true,
	},
	DiyStatusShipped: {
		DiyStatusCompleted: true,
		DiyStatusInReturn:  true,
	},
	DiyStatusInReturn: {
		DiyStatusCompleted: true,
		DiyStatusShipped:   true,
	},
}

// CanDiyTransit 校验DIY订单状态流转是否合法
func CanDiyTransit(from, to string) bool {
	if from == to {
		return false
	}
	allowed, ok := diyValidTransitions[from]
	if !ok {
		return false
	}
	return allowed[to]
}

// IsDiyTerminalStatus 是否终态
func IsDiyTerminalStatus(s string) bool {
	return s == DiyStatusCompleted || s == DiyStatusCancelled
}

const diyOrderTable = "askxuan_diy.diy_order"

var ErrDiyOrderStateConflict = errors.New("diy order state conflict")

// DiyOrder DIY订单表
type DiyOrder struct {
	Id                  int64   `db:"id" json:"id"`
	OrderNo             string  `db:"order_no" json:"orderNo"`
	UserId              string  `db:"user_id" json:"userId"`
	DesignId            int64   `db:"design_id" json:"designId"`
	MaterialFee         float64 `db:"material_fee" json:"materialFee"`
	BlessFee            float64 `db:"bless_fee" json:"blessFee"`
	TotalFee            float64 `db:"total_fee" json:"totalFee"`
	Status              string  `db:"status" json:"status"`
	PaymentStatus       string  `db:"payment_status" json:"paymentStatus"`
	AddressId           int64   `db:"address_id" json:"addressId"`
	Source              string  `db:"source" json:"source"`
	CreatorId           string  `db:"creator_id" json:"creatorId"`
	CreatorShareRate    float64 `db:"creator_share_rate" json:"creatorShareRate"`
	OriginalMaterialFee float64 `db:"original_material_fee" json:"originalMaterialFee"`
	PriceChanged        int     `db:"price_changed" json:"priceChanged"`
	DesignSnapshot      string  `db:"design_snapshot" json:"designSnapshot"`
	PricingSnapshot     string  `db:"pricing_snapshot" json:"pricingSnapshot"`
	CreateTime          string  `db:"create_time" json:"createTime"`
	UpdateTime          string  `db:"update_time" json:"updateTime"`
}

// Legacy orders predate immutable snapshots, so normalize their nullable fields
// at the query boundary instead of making every caller handle sql.NullString.
const diyOrderRows = "id,order_no,user_id,design_id,material_fee,bless_fee,total_fee,status,payment_status,address_id,COALESCE(source,''),COALESCE(creator_id,''),creator_share_rate,original_material_fee,price_changed,COALESCE(design_snapshot,''),COALESCE(pricing_snapshot,''),create_time,update_time"

// DiyOrderModel DIY订单模型接口
type DiyOrderModel interface {
	Insert(ctx context.Context, data *DiyOrder) (*DiyOrder, error)
	InsertSession(ctx context.Context, session sqlx.Session, data *DiyOrder) (*DiyOrder, error)
	FindOne(ctx context.Context, id int64) (*DiyOrder, error)
	FindByOrderNo(ctx context.Context, orderNo string) (*DiyOrder, error)
	FindListByUser(ctx context.Context, userId, status string, page, size int) ([]*DiyOrder, int64, error)
	FindListAdmin(ctx context.Context, status string, page, size int) ([]*DiyOrder, int64, error)
	UpdateStatus(ctx context.Context, id int64, status string) (*DiyOrder, error)
	UpdateStatusIfCurrent(ctx context.Context, id int64, currentStatus, targetStatus string) (*DiyOrder, error)
	CompleteMaking(ctx context.Context, id int64, blessServiceCode string) (*DiyOrder, *BlessingTask, error)
	CancelAndRestock(ctx context.Context, id int64) (*DiyOrder, error)
}

type defaultDiyOrderModel struct {
	conn sqlx.SqlConn
}

func NewDiyOrderModel(conn sqlx.SqlConn) DiyOrderModel {
	return &defaultDiyOrderModel{conn: conn}
}

func (m *defaultDiyOrderModel) Insert(ctx context.Context, data *DiyOrder) (*DiyOrder, error) {
	return m.InsertSession(ctx, m.conn, data)
}

func (m *defaultDiyOrderModel) InsertSession(ctx context.Context, session sqlx.Session, data *DiyOrder) (*DiyOrder, error) {
	if data.OrderNo == "" {
		data.OrderNo = "DIY" + time.Now().Format("20060102") + fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	}
	if data.Status == "" {
		data.Status = DiyStatusPendingReview
	}
	if data.PaymentStatus == "" {
		data.PaymentStatus = "pending"
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	data.CreateTime = now
	data.UpdateTime = now

	query := fmt.Sprintf(`INSERT INTO %s (order_no,user_id,design_id,material_fee,bless_fee,total_fee,status,payment_status,address_id,source,creator_id,creator_share_rate,original_material_fee,price_changed,design_snapshot,pricing_snapshot,create_time,update_time) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, diyOrderTable)
	result, err := session.ExecCtx(ctx, query, data.OrderNo, data.UserId, data.DesignId, data.MaterialFee, data.BlessFee, data.TotalFee, data.Status, data.PaymentStatus, data.AddressId, data.Source, data.CreatorId, data.CreatorShareRate, data.OriginalMaterialFee, data.PriceChanged, data.DesignSnapshot, data.PricingSnapshot, data.CreateTime, data.UpdateTime)
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

func (m *defaultDiyOrderModel) FindOne(ctx context.Context, id int64) (*DiyOrder, error) {
	var o DiyOrder
	query := fmt.Sprintf(`SELECT %s FROM %s WHERE id = ?`, diyOrderRows, diyOrderTable)
	err := m.conn.QueryRowCtx(ctx, &o, query, id)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (m *defaultDiyOrderModel) FindByOrderNo(ctx context.Context, orderNo string) (*DiyOrder, error) {
	var o DiyOrder
	query := fmt.Sprintf(`SELECT %s FROM %s WHERE order_no = ?`, diyOrderRows, diyOrderTable)
	err := m.conn.QueryRowCtx(ctx, &o, query, orderNo)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

func (m *defaultDiyOrderModel) FindListByUser(ctx context.Context, userId, status string, page, size int) ([]*DiyOrder, int64, error) {
	where, args := buildDiyOrderWhere(userId, status)
	return m.findList(ctx, where, args, page, size)
}

func (m *defaultDiyOrderModel) FindListAdmin(ctx context.Context, status string, page, size int) ([]*DiyOrder, int64, error) {
	where, args := buildDiyOrderWhere("", status)
	return m.findList(ctx, where, args, page, size)
}

func (m *defaultDiyOrderModel) findList(ctx context.Context, where string, args []interface{}, page, size int) ([]*DiyOrder, int64, error) {
	countQuery := fmt.Sprintf(`SELECT COUNT(1) FROM %s WHERE %s`, diyOrderTable, where)
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*DiyOrder{}, 0, nil
	}

	offset := (page - 1) * size
	listQuery := fmt.Sprintf(`SELECT %s FROM %s WHERE %s ORDER BY create_time DESC LIMIT ?, ?`, diyOrderRows, diyOrderTable, where)
	listArgs := append(args, offset, size)
	var list []*DiyOrder
	if err := m.conn.QueryRowsCtx(ctx, &list, listQuery, listArgs...); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *defaultDiyOrderModel) UpdateStatus(ctx context.Context, id int64, status string) (*DiyOrder, error) {
	query := fmt.Sprintf(`UPDATE %s SET status=?, update_time=? WHERE id=?`, diyOrderTable)
	_, err := m.conn.ExecCtx(ctx, query, status, time.Now().Format("2006-01-02 15:04:05"), id)
	if err != nil {
		return nil, err
	}
	return m.FindOne(ctx, id)
}

func (m *defaultDiyOrderModel) UpdateStatusIfCurrent(ctx context.Context, id int64, currentStatus, targetStatus string) (*DiyOrder, error) {
	query := fmt.Sprintf(`UPDATE %s SET status=?,update_time=? WHERE id=? AND status=?`, diyOrderTable)
	result, err := m.conn.ExecCtx(ctx, query, targetStatus, time.Now().Format("2006-01-02 15:04:05"), id, currentStatus)
	if err != nil {
		return nil, err
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		return nil, ErrDiyOrderStateConflict
	}
	return m.FindOne(ctx, id)
}

func (m *defaultDiyOrderModel) CompleteMaking(ctx context.Context, id int64, blessServiceCode string) (updated *DiyOrder, task *BlessingTask, err error) {
	err = m.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		var order DiyOrder
		query := fmt.Sprintf(`SELECT %s FROM %s WHERE id=? FOR UPDATE`, diyOrderRows, diyOrderTable)
		if queryErr := session.QueryRowCtx(ctx, &order, query, id); queryErr != nil {
			return queryErr
		}
		if order.Status != DiyStatusInMaking {
			return ErrDiyOrderStateConflict
		}

		targetStatus := DiyStatusAwaitingShipment
		if blessServiceCode != "" {
			var service ExtraService
			if queryErr := session.QueryRowCtx(ctx, &service,
				`SELECT id,code,name,temple_code,master_code,price,description,status,create_time FROM extra_service WHERE code=? AND status=? FOR UPDATE`,
				blessServiceCode, BlessingServiceStatusOnShelf); queryErr != nil {
				if errors.Is(queryErr, sqlx.ErrNotFound) {
					return ErrOrderBlessingUnavailable
				}
				return queryErr
			}

			now := time.Now().Format("2006-01-02 15:04:05")
			task = &BlessingTask{
				TaskNo:     newBlessingTaskNo(),
				DiyOrderNo: order.OrderNo,
				TempleCode: service.TempleCode,
				MasterCode: service.MasterCode,
				Status:     BlessingTaskStatusDispatched,
				AssignTime: now,
				CreateTime: now,
				UpdateTime: now,
			}
			result, insertErr := session.ExecCtx(ctx,
				fmt.Sprintf(`INSERT INTO %s(task_no,diy_order_no,temple_code,master_code,status,certificate_urls,assign_time,create_time,update_time) VALUES(?,?,?,?,?,'[]',?,?,?)`, blessingTaskTable),
				task.TaskNo, task.DiyOrderNo, task.TempleCode, task.MasterCode, task.Status, now, now, now)
			if insertErr != nil {
				return insertErr
			}
			task.Id, insertErr = result.LastInsertId()
			if insertErr != nil {
				return insertErr
			}
			targetStatus = DiyStatusAwaitingBlessing
		}

		order.UpdateTime = time.Now().Format("2006-01-02 15:04:05")
		result, updateErr := session.ExecCtx(ctx,
			fmt.Sprintf(`UPDATE %s SET status=?,update_time=? WHERE id=? AND status=?`, diyOrderTable),
			targetStatus, order.UpdateTime, order.Id, DiyStatusInMaking)
		if updateErr != nil {
			return updateErr
		}
		rows, rowsErr := result.RowsAffected()
		if rowsErr != nil || rows != 1 {
			return ErrDiyOrderStateConflict
		}
		order.Status = targetStatus
		updated = &order
		return nil
	})
	return updated, task, err
}

func (m *defaultDiyOrderModel) CancelAndRestock(ctx context.Context, id int64) (updated *DiyOrder, err error) {
	err = m.conn.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		var order DiyOrder
		query := fmt.Sprintf(`SELECT %s FROM %s WHERE id=? FOR UPDATE`, diyOrderRows, diyOrderTable)
		if queryErr := session.QueryRowCtx(ctx, &order, query, id); queryErr != nil {
			return queryErr
		}
		if order.Status != DiyStatusPendingReview {
			return ErrDiyOrderStateConflict
		}

		var items []*DiyOrderItem
		if queryErr := session.QueryRowsCtx(ctx, &items, `SELECT id,order_id,material_id,sku_id,material_name,spec,unit_price,quantity,subtype FROM askxuan_diy.diy_order_item WHERE order_id=? FOR UPDATE`, id); queryErr != nil {
			return queryErr
		}
		for _, item := range items {
			if _, execErr := session.ExecCtx(ctx, `UPDATE askxuan_diy.material SET stock=stock+? WHERE id=?`, item.Quantity, item.MaterialId); execErr != nil {
				return execErr
			}
			if item.SkuId > 0 {
				if _, execErr := session.ExecCtx(ctx, `UPDATE askxuan_diy.material_sku SET stock=stock+? WHERE id=?`, item.Quantity, item.SkuId); execErr != nil {
					return execErr
				}
			}
		}

		order.Status = DiyStatusCancelled
		order.UpdateTime = time.Now().Format("2006-01-02 15:04:05")
		result, execErr := session.ExecCtx(ctx, `UPDATE askxuan_diy.diy_order SET status=?,update_time=? WHERE id=? AND status=?`, order.Status, order.UpdateTime, id, DiyStatusPendingReview)
		if execErr != nil {
			return execErr
		}
		rows, rowsErr := result.RowsAffected()
		if rowsErr != nil || rows != 1 {
			return ErrDiyOrderStateConflict
		}
		updated = &order
		return nil
	})
	return updated, err
}

func buildDiyOrderWhere(userId, status string) (string, []interface{}) {
	where := "1=1"
	var args []interface{}
	if userId != "" {
		where += " AND user_id = ?"
		args = append(args, userId)
	}
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}
	return where, args
}

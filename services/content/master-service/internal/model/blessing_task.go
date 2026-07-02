package model

import (
	"context"
	"fmt"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ============ 加持任务状态机（修复 Gap-4/15，法师侧推进） ============

// 加持任务状态常量（与 temple-service 对齐）
const (
	BlessingTaskStatusDispatched  = "dispatched"  // 商城派单到寺院
	BlessingTaskStatusAssigned    = "assigned"    // 寺院分配法师
	BlessingTaskStatusAccepted    = "accepted"    // 法师接受任务
	BlessingTaskStatusInProgress  = "in_progress" // 法师开始加持
	BlessingTaskStatusCompleted   = "completed"   // 法师完成加持
	BlessingTaskStatusRejected    = "rejected"    // 法师拒绝任务
)

// blessingTaskValidTransitions 加持任务合法状态流转（法师侧）
var blessingTaskValidTransitions = map[string]map[string]bool{
	BlessingTaskStatusDispatched: {
		BlessingTaskStatusAssigned: true,
	},
	BlessingTaskStatusAssigned: {
		BlessingTaskStatusAccepted: true,
		BlessingTaskStatusRejected: true,
	},
	BlessingTaskStatusRejected: {
		BlessingTaskStatusAssigned: true,
	},
	BlessingTaskStatusAccepted: {
		BlessingTaskStatusInProgress: true,
	},
	BlessingTaskStatusInProgress: {
		BlessingTaskStatusCompleted: true,
	},
}

// CanTransitBlessingTask 校验加持任务状态流转是否合法
func CanTransitBlessingTask(from, to string) bool {
	if from == to {
		return false
	}
	allowed, ok := blessingTaskValidTransitions[from]
	if !ok {
		return false
	}
	return allowed[to]
}

// IsBlessingTaskTerminal 是否终态
func IsBlessingTaskTerminal(s string) bool {
	return s == BlessingTaskStatusCompleted
}

// ============ 加持任务 MySQL 存储（法师侧） ============

// BlessingTask 加持任务（对应 askxuan_diy.blessing_task）
type BlessingTask struct {
	Id              int64  `db:"id" json:"id"`
	TaskNo          string `db:"task_no" json:"taskNo"`
	DiyOrderNo      string `db:"diy_order_no" json:"diyOrderNo"`
	TempleCode      string `db:"temple_code" json:"templeCode"`
	MasterCode      string `db:"master_code" json:"masterCode"`
	Status          string `db:"status" json:"status"`
	CertificateUrls string `db:"certificate_urls" json:"certificateUrls"` // JSON 数组字符串
	AssignTime      string `db:"assign_time" json:"assignTime"`
	CompleteTime    string `db:"complete_time" json:"completeTime"`
	CreateTime      string `db:"create_time" json:"createTime"`
	UpdateTime      string `db:"update_time" json:"updateTime"`
}

// BlessingTaskModel 加持任务模型接口
type BlessingTaskModel interface {
	FindOne(ctx context.Context, id int64) (*BlessingTask, error)
	FindByTaskNo(ctx context.Context, taskNo string) (*BlessingTask, error)
	FindByMasterId(ctx context.Context, masterCode, status string, page, size int) ([]*BlessingTask, int64, error)
	UpdateStatus(ctx context.Context, id int64, status string) error
	UpdateComplete(ctx context.Context, id int64, certificateUrls string) error
}

type blessingTaskModel struct {
	conn  sqlx.SqlConn
	table string
}

// NewBlessingTaskModel 构造加持任务模型
func NewBlessingTaskModel(conn sqlx.SqlConn) BlessingTaskModel {
	return &blessingTaskModel{conn: conn, table: "askxuan_diy.blessing_task"}
}

const blessingTaskRows = "id, task_no, diy_order_no, temple_code, master_code, status, certificate_urls, assign_time, complete_time, create_time, update_time"

func (m *blessingTaskModel) FindOne(ctx context.Context, id int64) (*BlessingTask, error) {
	var task BlessingTask
	query := fmt.Sprintf("SELECT %s FROM %s WHERE id = ?", blessingTaskRows, m.table)
	err := m.conn.QueryRowCtx(ctx, &task, query, id)
	if err != nil {
		return nil, err
	}
	return &task, nil
}

// FindByTaskNo 按任务编号查找加持任务
func (m *blessingTaskModel) FindByTaskNo(ctx context.Context, taskNo string) (*BlessingTask, error) {
	var task BlessingTask
	query := fmt.Sprintf("SELECT %s FROM %s WHERE task_no = ? LIMIT 1", blessingTaskRows, m.table)
	err := m.conn.QueryRowCtx(ctx, &task, query, taskNo)
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (m *blessingTaskModel) FindByMasterId(ctx context.Context, masterCode, status string, page, size int) ([]*BlessingTask, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	offset := (page - 1) * size

	where := "WHERE master_code = ?"
	args := []interface{}{masterCode}
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}

	var total int64
	countQuery := fmt.Sprintf("SELECT COUNT(1) FROM %s %s", m.table, where)
	err := m.conn.QueryRowCtx(ctx, &total, countQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*BlessingTask{}, 0, nil
	}

	var list []*BlessingTask
	listQuery := fmt.Sprintf("SELECT %s FROM %s %s ORDER BY id DESC LIMIT %d, %d", blessingTaskRows, m.table, where, offset, size)
	err = m.conn.QueryRowsCtx(ctx, &list, listQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (m *blessingTaskModel) UpdateStatus(ctx context.Context, id int64, status string) error {
	query := fmt.Sprintf("UPDATE %s SET status = ? WHERE id = ?", m.table)
	_, err := m.conn.ExecCtx(ctx, query, status, id)
	return err
}

func (m *blessingTaskModel) UpdateComplete(ctx context.Context, id int64, certificateUrls string) error {
	query := fmt.Sprintf("UPDATE %s SET status = ?, certificate_urls = ?, complete_time = NOW() WHERE id = ?", m.table)
	_, err := m.conn.ExecCtx(ctx, query, BlessingTaskStatusCompleted, certificateUrls, id)
	return err
}

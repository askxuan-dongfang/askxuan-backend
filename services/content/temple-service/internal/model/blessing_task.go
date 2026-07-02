package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ============ 加持任务状态机（修复 Gap-3，参照 state-machines.md 第3节） ============

// 加持任务状态常量
const (
	BlessingTaskStatusDispatched  = "dispatched"  // 商城派单到寺院
	BlessingTaskStatusAssigned    = "assigned"    // 寺院分配法师
	BlessingTaskStatusAccepted    = "accepted"    // 法师接受任务
	BlessingTaskStatusInProgress  = "in_progress" // 法师开始加持
	BlessingTaskStatusCompleted   = "completed"   // 法师完成加持
	BlessingTaskStatusRejected    = "rejected"    // 法师拒绝任务
)

// blessingTaskValidTransitions 加持任务合法状态流转
// key = 当前状态，value = 允许流转到的目标状态集合
var blessingTaskValidTransitions = map[string]map[string]bool{
	BlessingTaskStatusDispatched: {
		BlessingTaskStatusAssigned: true,
	},
	BlessingTaskStatusAssigned: {
		BlessingTaskStatusAccepted: true,
		BlessingTaskStatusRejected: true,
	},
	BlessingTaskStatusRejected: {
		BlessingTaskStatusAssigned: true, // 寺院重新分配法师
	},
	BlessingTaskStatusAccepted: {
		BlessingTaskStatusInProgress: true,
	},
	BlessingTaskStatusInProgress: {
		BlessingTaskStatusCompleted: true,
	},
	// completed 为终态
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

// ============ 加持任务 MySQL 存储 ============

// blessingTaskTable 加持任务表（位于 askxuan_diy 库）
const blessingTaskTable = "askxuan_diy.blessing_task"

// BlessingTask 加持任务
type BlessingTask struct {
	Id              int64    `db:"id" json:"id"`
	TaskNo          string   `db:"task_no" json:"taskNo"`
	DiyOrderNo      string   `db:"diy_order_no" json:"diyOrderNo"`
	TempleCode      string   `db:"temple_code" json:"templeCode"`
	MasterCode      string   `db:"master_code" json:"masterCode"`
	Status          string   `db:"status" json:"status"`
	CertificateUrls []string `json:"certificateUrls"` // DB 中为 JSON 字符串
	AssignTime      string   `db:"assign_time" json:"assignTime"`
	CompleteTime    string   `db:"complete_time" json:"completeTime"`
	CreateTime      string   `db:"create_time" json:"createTime"`
	UpdateTime      string   `db:"update_time" json:"updateTime"`
}

// blessingTaskRow 加持任务 DB 行结构（CertificateUrls 为 JSON 字符串）
type blessingTaskRow struct {
	Id              int64  `db:"id"`
	TaskNo          string `db:"task_no"`
	DiyOrderNo      string `db:"diy_order_no"`
	TempleCode      string `db:"temple_code"`
	MasterCode      string `db:"master_code"`
	Status          string `db:"status"`
	CertificateUrls string `db:"certificate_urls"`
	AssignTime      string `db:"assign_time"`
	CompleteTime    string `db:"complete_time"`
	CreateTime      string `db:"create_time"`
	UpdateTime      string `db:"update_time"`
}

// BlessingTaskModel 加持任务模型接口
type BlessingTaskModel interface {
	Insert(ctx context.Context, data *BlessingTask) (int64, error)
	FindOne(ctx context.Context, id int64) (*BlessingTask, error)
	FindByTempleId(ctx context.Context, templeCode, status string, page, size int) ([]*BlessingTask, int64, error)
	FindList(ctx context.Context, templeCode, status string, page, size int) ([]*BlessingTask, int64, error)
	Update(ctx context.Context, data *BlessingTask) error
	UpdateStatus(ctx context.Context, id int64, status string) error
	Assign(ctx context.Context, id int64, masterCode string) (*BlessingTask, error)
	Delete(ctx context.Context, id int64) error
}

type defaultBlessingTaskModel struct {
	conn sqlx.SqlConn
}

// NewBlessingTaskModel 构造加持任务模型
func NewBlessingTaskModel(conn sqlx.SqlConn) BlessingTaskModel {
	return &defaultBlessingTaskModel{conn: conn}
}

// Insert 新增加持任务，返回自增 ID
func (m *defaultBlessingTaskModel) Insert(ctx context.Context, data *BlessingTask) (int64, error) {
	certUrlsJSON := "[]"
	if len(data.CertificateUrls) > 0 {
		if b, err := json.Marshal(data.CertificateUrls); err == nil {
			certUrlsJSON = string(b)
		}
	}
	if data.Status == "" {
		data.Status = BlessingTaskStatusDispatched
	}
	query := fmt.Sprintf(`INSERT INTO %s (task_no, diy_order_no, temple_code, master_code, status, certificate_urls) VALUES (?, ?, ?, ?, ?, ?)`,
		blessingTaskTable)
	res, err := m.conn.ExecCtx(ctx, query,
		data.TaskNo, data.DiyOrderNo, data.TempleCode,
		data.MasterCode, data.Status, certUrlsJSON)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	return id, nil
}

// FindOne 按 ID 查询加持任务
func (m *defaultBlessingTaskModel) FindOne(ctx context.Context, id int64) (*BlessingTask, error) {
	query := fmt.Sprintf(`SELECT id, task_no, diy_order_no, temple_code, master_code, status, certificate_urls, assign_time, complete_time, create_time, update_time FROM %s WHERE id = ?`,
		blessingTaskTable)
	var row blessingTaskRow
	if err := m.conn.QueryRowCtx(ctx, &row, query, id); err != nil {
		return nil, err
	}
	return rowToBlessingTask(&row), nil
}

// FindByTempleId 查询寺院的加持任务，支持按 status 筛选 + 分页
func (m *defaultBlessingTaskModel) FindByTempleId(ctx context.Context, templeCode, status string, page, size int) ([]*BlessingTask, int64, error) {
	return m.FindList(ctx, templeCode, status, page, size)
}

// FindList 查询加持任务列表，支持按 templeCode/status 筛选 + 分页
func (m *defaultBlessingTaskModel) FindList(ctx context.Context, templeCode, status string, page, size int) ([]*BlessingTask, int64, error) {
	where := "1=1"
	var args []interface{}
	if templeCode != "" {
		where += " AND temple_code = ?"
		args = append(args, templeCode)
	}
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}

	countQuery := fmt.Sprintf(`SELECT COUNT(1) FROM %s WHERE %s`, blessingTaskTable, where)
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, countQuery, args...); err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*BlessingTask{}, 0, nil
	}

	offset := (page - 1) * size
	listQuery := fmt.Sprintf(`SELECT id, task_no, diy_order_no, temple_code, master_code, status, certificate_urls, assign_time, complete_time, create_time, update_time FROM %s WHERE %s ORDER BY id DESC LIMIT ?, ?`,
		blessingTaskTable, where)
	listArgs := append(args, offset, size)
	var rows []*blessingTaskRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, listQuery, listArgs...); err != nil {
		return nil, 0, err
	}
	list := make([]*BlessingTask, 0, len(rows))
	for _, r := range rows {
		list = append(list, rowToBlessingTask(r))
	}
	return list, total, nil
}

// Update 更新加持任务
func (m *defaultBlessingTaskModel) Update(ctx context.Context, data *BlessingTask) error {
	certUrlsJSON := "[]"
	if len(data.CertificateUrls) > 0 {
		if b, err := json.Marshal(data.CertificateUrls); err == nil {
			certUrlsJSON = string(b)
		}
	}
	query := fmt.Sprintf(`UPDATE %s SET task_no = ?, diy_order_no = ?, temple_code = ?, master_code = ?, status = ?, certificate_urls = ? WHERE id = ?`,
		blessingTaskTable)
	_, err := m.conn.ExecCtx(ctx, query,
		data.TaskNo, data.DiyOrderNo, data.TempleCode,
		data.MasterCode, data.Status, certUrlsJSON, data.Id)
	return err
}

// UpdateStatus 更新任务状态
func (m *defaultBlessingTaskModel) UpdateStatus(ctx context.Context, id int64, status string) error {
	query := fmt.Sprintf(`UPDATE %s SET status = ? WHERE id = ?`, blessingTaskTable)
	_, err := m.conn.ExecCtx(ctx, query, status, id)
	return err
}

// Assign 寺院分配法师（dispatched/rejected → assigned）
// 校验状态流转合法性，更新 master_code/status/assign_time，返回更新后的任务
func (m *defaultBlessingTaskModel) Assign(ctx context.Context, id int64, masterCode string) (*BlessingTask, error) {
	task, err := m.FindOne(ctx, id)
	if err != nil {
		return nil, err
	}
	if !CanTransitBlessingTask(task.Status, BlessingTaskStatusAssigned) {
		return nil, errors.New("加持任务状态不允许分配法师")
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	query := fmt.Sprintf(`UPDATE %s SET master_code = ?, status = ?, assign_time = ? WHERE id = ?`, blessingTaskTable)
	if _, err := m.conn.ExecCtx(ctx, query, masterCode, BlessingTaskStatusAssigned, now, id); err != nil {
		return nil, err
	}
	task.MasterCode = masterCode
	task.Status = BlessingTaskStatusAssigned
	task.AssignTime = now
	return task, nil
}

// Delete 删除加持任务
func (m *defaultBlessingTaskModel) Delete(ctx context.Context, id int64) error {
	query := fmt.Sprintf(`DELETE FROM %s WHERE id = ?`, blessingTaskTable)
	_, err := m.conn.ExecCtx(ctx, query, id)
	return err
}

// rowToBlessingTask 将 DB 行结构转为 BlessingTask（CertificateUrls JSON 反序列化）
func rowToBlessingTask(row *blessingTaskRow) *BlessingTask {
	var urls []string
	if row.CertificateUrls != "" {
		_ = json.Unmarshal([]byte(row.CertificateUrls), &urls)
	}
	if urls == nil {
		urls = []string{}
	}
	return &BlessingTask{
		Id:              row.Id,
		TaskNo:          row.TaskNo,
		DiyOrderNo:      row.DiyOrderNo,
		TempleCode:      row.TempleCode,
		MasterCode:      row.MasterCode,
		Status:          row.Status,
		CertificateUrls: urls,
		AssignTime:      row.AssignTime,
		CompleteTime:    row.CompleteTime,
		CreateTime:      row.CreateTime,
		UpdateTime:      row.UpdateTime,
	}
}

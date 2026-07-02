package model

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// 加持任务状态常量（参照 state-machines.md 加持任务状态机，与 temple/master-service 对齐）
const (
	BlessingTaskStatusDispatched = "dispatched"  // 已派单
	BlessingTaskStatusAssigned   = "assigned"    // 已分配法师
	BlessingTaskStatusAccepted   = "accepted"    // 法师已接受
	BlessingTaskStatusInProgress = "in_progress" // 加持中
	BlessingTaskStatusCompleted  = "completed"   // 已完成
	BlessingTaskStatusRejected   = "rejected"    // 已拒绝
)

// blessingTaskValidTransitions 加持任务合法状态流转（参照 state-machines.md 3.2）
var blessingTaskValidTransitions = map[string]map[string]bool{
	BlessingTaskStatusDispatched: {
		BlessingTaskStatusAssigned: true,
	},
	BlessingTaskStatusAssigned: {
		BlessingTaskStatusAccepted: true,
		BlessingTaskStatusRejected: true,
	},
	BlessingTaskStatusRejected: {
		BlessingTaskStatusAssigned: true, // 重新分配
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

const blessingTaskTable = "askxuan_diy.blessing_task"

// BlessingTask 加持任务表（对外使用，CertificateUrls 为切片）
type BlessingTask struct {
	Id              int64    `db:"id" json:"id"`
	TaskNo          string   `db:"task_no" json:"taskNo"`
	DiyOrderNo      string   `db:"diy_order_no" json:"diyOrderNo"`
	TempleCode      string   `db:"temple_code" json:"templeCode"`
	MasterCode      string   `db:"master_code" json:"masterCode"`
	Status          string   `db:"status" json:"status"`
	CertificateUrls []string `db:"-" json:"certificateUrls"`
	AssignTime      string   `db:"assign_time" json:"assignTime"`
	CompleteTime    string   `db:"complete_time" json:"completeTime"`
	CreateTime      string   `db:"create_time" json:"createTime"`
	UpdateTime      string   `db:"update_time" json:"updateTime"`
}

// blessingTaskRow DB 行结构（certificate_urls 为 string）
// AssignTime/CompleteTime 使用 sql.NullString 处理 NULL 值
type blessingTaskRow struct {
	Id              int64           `db:"id"`
	TaskNo          string          `db:"task_no"`
	DiyOrderNo      string          `db:"diy_order_no"`
	TempleCode      string          `db:"temple_code"`
	MasterCode      string          `db:"master_code"`
	Status          string          `db:"status"`
	CertificateUrls string          `db:"certificate_urls"`
	AssignTime      sql.NullString  `db:"assign_time"`
	CompleteTime    sql.NullString  `db:"complete_time"`
	CreateTime      string          `db:"create_time"`
	UpdateTime      string          `db:"update_time"`
}

// BlessingTaskModel 加持任务模型接口
type BlessingTaskModel interface {
	Insert(ctx context.Context, data *BlessingTask) (*BlessingTask, error)
	FindOne(ctx context.Context, id int64) (*BlessingTask, error)
	FindByDiyOrderNo(ctx context.Context, diyOrderNo string) (*BlessingTask, error)
	UpdateStatus(ctx context.Context, id int64, status string) (*BlessingTask, error)
	UpdateCertificate(ctx context.Context, id int64, certificateUrls []string) error
}

type defaultBlessingTaskModel struct {
	conn sqlx.SqlConn
}

func NewBlessingTaskModel(conn sqlx.SqlConn) BlessingTaskModel {
	return &defaultBlessingTaskModel{conn: conn}
}

func (m *defaultBlessingTaskModel) Insert(ctx context.Context, data *BlessingTask) (*BlessingTask, error) {
	if data.TaskNo == "" {
		data.TaskNo = "BT" + time.Now().Format("20060102") + fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	}
	if data.Status == "" {
		data.Status = BlessingTaskStatusDispatched
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	data.CreateTime = now
	data.UpdateTime = now
	if data.AssignTime == "" {
		data.AssignTime = now
	}
	certJSON := urlsToJSON(data.CertificateUrls)

	// complete_time 为空时传 nil（MySQL DATETIME 不接受空字符串）
	var completeTimeArg interface{}
	if data.CompleteTime == "" {
		completeTimeArg = nil
	} else {
		completeTimeArg = data.CompleteTime
	}

	query := fmt.Sprintf(`INSERT INTO %s (task_no, diy_order_no, temple_code, master_code, status, certificate_urls, assign_time, complete_time, create_time, update_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, blessingTaskTable)
	result, err := m.conn.ExecCtx(ctx, query, data.TaskNo, data.DiyOrderNo, data.TempleCode, data.MasterCode, data.Status, certJSON, data.AssignTime, completeTimeArg, data.CreateTime, data.UpdateTime)
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

func (m *defaultBlessingTaskModel) FindOne(ctx context.Context, id int64) (*BlessingTask, error) {
	var row blessingTaskRow
	query := fmt.Sprintf(`SELECT id, task_no, diy_order_no, temple_code, master_code, status, certificate_urls, assign_time, complete_time, create_time, update_time FROM %s WHERE id = ?`, blessingTaskTable)
	err := m.conn.QueryRowCtx(ctx, &row, query, id)
	if err != nil {
		return nil, err
	}
	return rowToBlessingTask(&row), nil
}

func (m *defaultBlessingTaskModel) FindByDiyOrderNo(ctx context.Context, diyOrderNo string) (*BlessingTask, error) {
	var row blessingTaskRow
	query := fmt.Sprintf(`SELECT id, task_no, diy_order_no, temple_code, master_code, status, certificate_urls, assign_time, complete_time, create_time, update_time FROM %s WHERE diy_order_no = ?`, blessingTaskTable)
	err := m.conn.QueryRowCtx(ctx, &row, query, diyOrderNo)
	if err != nil {
		return nil, err
	}
	return rowToBlessingTask(&row), nil
}

func (m *defaultBlessingTaskModel) UpdateStatus(ctx context.Context, id int64, status string) (*BlessingTask, error) {
	now := time.Now().Format("2006-01-02 15:04:05")
	// complete_time 仅在 completed 状态时设置，其他状态传 nil 保持 NULL
	var completeTimeArg interface{}
	if status == BlessingTaskStatusCompleted {
		completeTimeArg = now
	} else {
		completeTimeArg = nil
	}
	query := fmt.Sprintf(`UPDATE %s SET status=?, complete_time=?, update_time=? WHERE id=?`, blessingTaskTable)
	_, err := m.conn.ExecCtx(ctx, query, status, completeTimeArg, now, id)
	if err != nil {
		return nil, err
	}
	return m.FindOne(ctx, id)
}

func (m *defaultBlessingTaskModel) UpdateCertificate(ctx context.Context, id int64, certificateUrls []string) error {
	certJSON := urlsToJSON(certificateUrls)
	now := time.Now().Format("2006-01-02 15:04:05")
	query := fmt.Sprintf(`UPDATE %s SET certificate_urls=?, update_time=? WHERE id=?`, blessingTaskTable)
	_, err := m.conn.ExecCtx(ctx, query, certJSON, now, id)
	return err
}

// rowToBlessingTask 将 DB 行转换为对外结构体
func rowToBlessingTask(row *blessingTaskRow) *BlessingTask {
	return &BlessingTask{
		Id:              row.Id,
		TaskNo:          row.TaskNo,
		DiyOrderNo:      row.DiyOrderNo,
		TempleCode:      row.TempleCode,
		MasterCode:      row.MasterCode,
		Status:          row.Status,
		CertificateUrls: jsonToUrls(row.CertificateUrls),
		AssignTime:      row.AssignTime.String,
		CompleteTime:    row.CompleteTime.String,
		CreateTime:      row.CreateTime,
		UpdateTime:      row.UpdateTime,
	}
}

func urlsToJSON(urls []string) string {
	if len(urls) == 0 {
		return "[]"
	}
	b, err := json.Marshal(urls)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func jsonToUrls(s string) []string {
	if s == "" {
		return []string{}
	}
	var urls []string
	if err := json.Unmarshal([]byte(s), &urls); err != nil {
		return []string{}
	}
	return urls
}

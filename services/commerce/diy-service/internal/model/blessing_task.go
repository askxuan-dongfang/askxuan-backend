package model

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
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
	Id              int64          `db:"id"`
	TaskNo          string         `db:"task_no"`
	DiyOrderNo      string         `db:"diy_order_no"`
	TempleCode      string         `db:"temple_code"`
	MasterCode      string         `db:"master_code"`
	Status          string         `db:"status"`
	CertificateUrls sql.NullString `db:"certificate_urls"`
	AssignTime      sql.NullString `db:"assign_time"`
	CompleteTime    sql.NullString `db:"complete_time"`
	CreateTime      string         `db:"create_time"`
	UpdateTime      string         `db:"update_time"`
}

// BlessingTaskModel 加持任务模型接口
type BlessingTaskModel interface {
	Insert(ctx context.Context, data *BlessingTask) (*BlessingTask, error)
	FindOne(ctx context.Context, id int64) (*BlessingTask, error)
	FindByDiyOrderNo(ctx context.Context, diyOrderNo string) (*BlessingTask, error)
	FindByTaskNo(ctx context.Context, taskNo string) (*BlessingTask, error)
	FindList(ctx context.Context, masterCode, templeCode, status string, page, size int) ([]*BlessingTask, int64, error)
	UpdateStatus(ctx context.Context, id int64, status string) (*BlessingTask, error)
	UpdateCertificate(ctx context.Context, id int64, certificateUrls []string) error
	UpdateComplete(ctx context.Context, id int64, certificateUrlsJSON string) error
	Assign(ctx context.Context, id int64, masterCode string) (*BlessingTask, error)
}

type defaultBlessingTaskModel struct {
	conn sqlx.SqlConn
}

func NewBlessingTaskModel(conn sqlx.SqlConn) BlessingTaskModel {
	return &defaultBlessingTaskModel{conn: conn}
}

func newBlessingTaskNo() string {
	return "BT" + time.Now().Format("20060102") + fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
}

func (m *defaultBlessingTaskModel) Insert(ctx context.Context, data *BlessingTask) (*BlessingTask, error) {
	if data.TaskNo == "" {
		data.TaskNo = newBlessingTaskNo()
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

func (m *defaultBlessingTaskModel) FindByTaskNo(ctx context.Context, taskNo string) (*BlessingTask, error) {
	var row blessingTaskRow
	query := fmt.Sprintf(`SELECT id, task_no, diy_order_no, temple_code, master_code, status, certificate_urls, assign_time, complete_time, create_time, update_time FROM %s WHERE task_no = ? LIMIT 1`, blessingTaskTable)
	err := m.conn.QueryRowCtx(ctx, &row, query, taskNo)
	if err != nil {
		return nil, err
	}
	return rowToBlessingTask(&row), nil
}

// FindList 查询加持任务列表，支持按 masterCode/templeCode/status 筛选 + 分页
func (m *defaultBlessingTaskModel) FindList(ctx context.Context, masterCode, templeCode, status string, page, size int) ([]*BlessingTask, int64, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 {
		size = 20
	}
	offset := (page - 1) * size

	where := "1=1"
	var args []interface{}
	if masterCode != "" {
		where += " AND master_code = ?"
		args = append(args, masterCode)
	}
	if templeCode != "" {
		where += " AND temple_code = ?"
		args = append(args, templeCode)
	}
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}

	var total int64
	countQuery := fmt.Sprintf(`SELECT COUNT(1) FROM %s WHERE %s`, blessingTaskTable, where)
	err := m.conn.QueryRowCtx(ctx, &total, countQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	if total == 0 {
		return []*BlessingTask{}, 0, nil
	}

	var rows []*blessingTaskRow
	listQuery := fmt.Sprintf(`SELECT id, task_no, diy_order_no, temple_code, master_code, status, certificate_urls, assign_time, complete_time, create_time, update_time FROM %s WHERE %s ORDER BY id DESC LIMIT ?, ?`, blessingTaskTable, where)
	listArgs := append(args, offset, size)
	err = m.conn.QueryRowsCtx(ctx, &rows, listQuery, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	list := make([]*BlessingTask, 0, len(rows))
	for _, r := range rows {
		list = append(list, rowToBlessingTask(r))
	}
	return list, total, nil
}

// UpdateStatus locks the task and advances the customer order in the same transaction.
func (m *defaultBlessingTaskModel) UpdateStatus(ctx context.Context, id int64, status string) (*BlessingTask, error) {
	if status == BlessingTaskStatusCompleted || status == BlessingTaskStatusAssigned {
		return nil, fmt.Errorf("请使用分配或完成接口")
	}
	err := m.conn.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var row blessingTaskRow
		if err := tx.QueryRowCtx(ctx, &row, "SELECT * FROM "+blessingTaskTable+" WHERE id=? FOR UPDATE", id); err != nil {
			return err
		}
		if !CanTransitBlessingTask(row.Status, status) {
			return fmt.Errorf("任务状态已变化，请刷新后重试")
		}
		if status == BlessingTaskStatusInProgress {
			result, err := tx.ExecCtx(ctx, "UPDATE askxuan_diy.diy_order SET status=?,update_time=NOW() WHERE order_no=? AND status=?", DiyStatusBlessingInProgress, row.DiyOrderNo, DiyStatusAwaitingBlessing)
			if err != nil {
				return err
			}
			affected, err := result.RowsAffected()
			if err != nil {
				return err
			}
			if affected != 1 {
				return fmt.Errorf("订单状态不允许开始开光")
			}
		}
		_, err := tx.ExecCtx(ctx, "UPDATE "+blessingTaskTable+" SET status=?,update_time=NOW() WHERE id=?", status, id)
		return err
	})
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

// UpdateComplete 完成加持任务：设置状态为 completed + 证书 URL + complete_time
func (m *defaultBlessingTaskModel) UpdateComplete(ctx context.Context, id int64, certificateUrlsJSON string) error {
	if err := validateCertificateURLs(certificateUrlsJSON); err != nil {
		return err
	}
	return m.conn.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var row blessingTaskRow
		if err := tx.QueryRowCtx(ctx, &row, "SELECT * FROM "+blessingTaskTable+" WHERE id=? FOR UPDATE", id); err != nil {
			return err
		}
		if !CanTransitBlessingTask(row.Status, BlessingTaskStatusCompleted) {
			return fmt.Errorf("任务状态不允许完成开光")
		}
		result, err := tx.ExecCtx(ctx, "UPDATE askxuan_diy.diy_order SET status=?,update_time=NOW() WHERE order_no=? AND status IN (?,?)", DiyStatusAwaitingShipment, row.DiyOrderNo, DiyStatusAwaitingBlessing, DiyStatusBlessingInProgress)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected != 1 {
			return fmt.Errorf("订单状态不允许完成开光")
		}
		_, err = tx.ExecCtx(ctx, "UPDATE "+blessingTaskTable+" SET status=?,certificate_urls=?,complete_time=NOW(),update_time=NOW() WHERE id=?", BlessingTaskStatusCompleted, certificateUrlsJSON, id)
		return err
	})
}

func validateCertificateURLs(raw string) error {
	var urls []string
	if json.Unmarshal([]byte(raw), &urls) != nil || len(urls) == 0 || len(urls) > 9 {
		return fmt.Errorf("请上传 1 至 9 张开光凭证")
	}
	for _, value := range urls {
		u, err := url.Parse(strings.TrimSpace(value))
		if err != nil || u == nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil {
			return fmt.Errorf("开光凭证地址无效")
		}
	}
	return nil
}

// Assign validates both task state and the selected master's temple membership.
func (m *defaultBlessingTaskModel) Assign(ctx context.Context, id int64, masterCode string) (*BlessingTask, error) {
	err := m.conn.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var row blessingTaskRow
		if err := tx.QueryRowCtx(ctx, &row, "SELECT * FROM "+blessingTaskTable+" WHERE id=? FOR UPDATE", id); err != nil {
			return err
		}
		if !CanTransitBlessingTask(row.Status, BlessingTaskStatusAssigned) {
			return fmt.Errorf("任务状态不允许分配法师")
		}
		var eligible int
		if row.TempleCode != "" {
			if err := tx.QueryRowCtx(ctx, &eligible, "SELECT COUNT(*) FROM askxuan_master.master WHERE code=? AND temple_code=? AND manage_by='temple' AND platform_status='normal' AND auth_status='已认证'", masterCode, row.TempleCode); err != nil {
				return err
			}
		} else {
			// A direct order belongs to the customer-selected independent master.
			if masterCode != row.MasterCode || masterCode == "" {
				return fmt.Errorf("不能更换用户所选法师")
			}
			if err := tx.QueryRowCtx(ctx, &eligible, "SELECT COUNT(*) FROM askxuan_master.master WHERE code=? AND manage_by='platform' AND platform_status='normal' AND auth_status='已认证'", masterCode); err != nil {
				return err
			}
		}
		if eligible != 1 {
			return fmt.Errorf("请选择本寺院可接单的法师")
		}
		_, err := tx.ExecCtx(ctx, "UPDATE "+blessingTaskTable+" SET master_code=?,status=?,assign_time=NOW(),update_time=NOW() WHERE id=?", masterCode, BlessingTaskStatusAssigned, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	return m.FindOne(ctx, id)
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
		CertificateUrls: jsonToUrls(row.CertificateUrls.String),
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

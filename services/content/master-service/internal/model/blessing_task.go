package model

import (
	"context"
	"encoding/json"

	"github.com/askxuan/master-service/rpc/diy"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

// ============ 加持任务远程访问（通过 zrpc 调用 diy-service） ============

// BlessingTask 加持任务（对应 askxuan_diy.blessing_task）
type BlessingTask struct {
	Id              int64  `db:"id" json:"id"`
	TaskNo          string `db:"task_no" json:"taskNo"`
	DiyOrderNo      string `db:"diy_order_no" json:"diyOrderNo"`
	TempleCode      string `db:"temple_code" json:"templeCode"`
	MasterCode      string `db:"master_code" json:"masterCode"`
	Status          string `db:"status" json:"status"`
	CertificateUrls []string `db:"certificate_urls" json:"certificateUrls"` // RPC 传输为 JSON 字符串，本地转为切片
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

// remoteBlessingTaskModel 通过 zrpc 调用 diy-service 访问 blessing_task
type remoteBlessingTaskModel struct {
	client diy.DiyServiceClient
}

// NewBlessingTaskModel 构造加持任务模型（通过 zrpc client 调用 diy-service）
func NewBlessingTaskModel(client diy.DiyServiceClient) BlessingTaskModel {
	return &remoteBlessingTaskModel{client: client}
}

// rpcToModel 将 rpc diy.BlessingTask 转为本地 model.BlessingTask
// certificate_urls 在 RPC 中为 JSON 字符串，这里反序列化为 []string
func rpcToModel(t *diy.BlessingTask) *BlessingTask {
	return &BlessingTask{
		Id:              t.Id,
		TaskNo:          t.TaskNo,
		DiyOrderNo:      t.DiyOrderNo,
		TempleCode:      t.TempleCode,
		MasterCode:      t.MasterCode,
		Status:          t.Status,
		CertificateUrls: jsonStrToUrls(t.CertificateUrls),
		AssignTime:      t.AssignTime,
		CompleteTime:    t.CompleteTime,
		CreateTime:      t.CreateTime,
		UpdateTime:      t.UpdateTime,
	}
}

// jsonStrToUrls 将 JSON 数组字符串反序列化为 []string
func jsonStrToUrls(s string) []string {
	if s == "" {
		return []string{}
	}
	var urls []string
	if err := json.Unmarshal([]byte(s), &urls); err != nil {
		return []string{}
	}
	if urls == nil {
		urls = []string{}
	}
	return urls
}

// wrapRpcError 将 gRPC NotFound 错误转为 sqlx.ErrNotFound，保持调用方错误处理兼容
func wrapRpcError(err error) error {
	if err == nil {
		return nil
	}
	if status.Code(err) == codes.NotFound {
		return sqlx.ErrNotFound
	}
	return err
}

func (m *remoteBlessingTaskModel) FindOne(ctx context.Context, id int64) (*BlessingTask, error) {
	resp, err := m.client.GetBlessingTask(ctx, &diy.GetBlessingTaskReq{Id: id})
	if err != nil {
		return nil, wrapRpcError(err)
	}
	return rpcToModel(resp), nil
}

func (m *remoteBlessingTaskModel) FindByTaskNo(ctx context.Context, taskNo string) (*BlessingTask, error) {
	resp, err := m.client.GetBlessingTaskByTaskNo(ctx, &diy.GetBlessingTaskByTaskNoReq{TaskNo: taskNo})
	if err != nil {
		return nil, wrapRpcError(err)
	}
	return rpcToModel(resp), nil
}

func (m *remoteBlessingTaskModel) FindByMasterId(ctx context.Context, masterCode, status string, page, size int) ([]*BlessingTask, int64, error) {
	resp, err := m.client.ListBlessingTasks(ctx, &diy.ListBlessingTasksReq{
		MasterCode: masterCode,
		Status:     status,
		Page:       int64(page),
		PageSize:   int64(size),
	})
	if err != nil {
		return nil, 0, wrapRpcError(err)
	}
	list := make([]*BlessingTask, 0, len(resp.Tasks))
	for _, t := range resp.Tasks {
		list = append(list, rpcToModel(t))
	}
	return list, resp.Total, nil
}

func (m *remoteBlessingTaskModel) UpdateStatus(ctx context.Context, id int64, status string) error {
	_, err := m.client.UpdateBlessingTaskStatus(ctx, &diy.UpdateBlessingTaskStatusReq{
		Id:     id,
		Status: status,
	})
	return wrapRpcError(err)
}

func (m *remoteBlessingTaskModel) UpdateComplete(ctx context.Context, id int64, certificateUrls string) error {
	_, err := m.client.CompleteBlessingTask(ctx, &diy.CompleteBlessingTaskReq{
		Id:              id,
		CertificateUrls: certificateUrls,
	})
	return wrapRpcError(err)
}

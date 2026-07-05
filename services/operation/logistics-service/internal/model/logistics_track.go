package model

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// 物流轨迹状态常量（参照 state-machines.md 第4节 + 本服务扩展）
const (
	TrackStatusPending   = "pending"    // 待揽收
	TrackStatusInTransit = "in_transit" // 运输中
	TrackStatusDelivered = "delivered"  // 已派送
	TrackStatusSigned    = "signed"     // 已签收（终态）
)

// 业务类型
const (
	BizTypeOrder = "order"
	BizTypeDiy   = "diy"
)

// trackTransitions 物流轨迹状态机合法流转
var trackTransitions = map[string]map[string]bool{
	TrackStatusPending: {
		TrackStatusInTransit: true,
	},
	TrackStatusInTransit: {
		TrackStatusDelivered: true,
	},
	TrackStatusDelivered: {
		TrackStatusSigned: true,
	},
}

// CanTransitTrack 校验物流轨迹状态流转是否合法
func CanTransitTrack(from, to string) bool {
	if from == to {
		return false
	}
	allowed, ok := trackTransitions[from]
	if !ok {
		return false
	}
	return allowed[to]
}

// IsTrackTerminalStatus 是否终态
func IsTrackTerminalStatus(s string) bool {
	return s == TrackStatusSigned
}

// TrackTrace 物流轨迹节点
type TrackTrace struct {
	Time string `json:"time"`
	Desc string `json:"desc"`
}

const logisticsTrackTable = "logistics_track"

// LogisticsTrack 物流追踪结构体（对外使用，Traces 为切片）
type LogisticsTrack struct {
	Id           int64        `db:"id" json:"id"`
	TrackingNo   string       `db:"tracking_no" json:"trackingNo"`
	ExpressCode  string       `db:"express_code" json:"expressCode"`
	ExpressName  string       `db:"express_name" json:"expressName"`
	BizType      string       `db:"biz_type" json:"bizType"`
	BizNo        string       `db:"biz_no" json:"bizNo"`
	Status       string       `db:"status" json:"status"`
	Traces       []TrackTrace `db:"-" json:"traces"`
	LastSyncTime string       `db:"last_sync_time" json:"lastSyncTime"`
	CreateTime   string       `db:"create_time" json:"createTime"`
	UpdateTime   string       `db:"update_time" json:"updateTime"`
}

// logisticsTrackRow DB 行结构（traces 为 string）
type logisticsTrackRow struct {
	Id           int64  `db:"id"`
	TrackingNo   string `db:"tracking_no"`
	ExpressCode  string `db:"express_code"`
	ExpressName  string `db:"express_name"`
	BizType      string `db:"biz_type"`
	BizNo        string `db:"biz_no"`
	Status       string `db:"status"`
	Traces       string `db:"traces"`
	LastSyncTime string `db:"last_sync_time"`
	CreateTime   string `db:"create_time"`
	UpdateTime   string `db:"update_time"`
}

// LogisticsTrackModel 物流轨迹模型接口
type LogisticsTrackModel interface {
	Insert(ctx context.Context, data *LogisticsTrack) (*LogisticsTrack, error)
	FindByTrackingNo(ctx context.Context, trackingNo string) (*LogisticsTrack, error)
	FindNonTerminal(ctx context.Context) ([]*LogisticsTrack, error)
	Update(ctx context.Context, data *LogisticsTrack) error
}

type defaultLogisticsTrackModel struct {
	conn sqlx.SqlConn
}

func NewLogisticsTrackModel(conn sqlx.SqlConn) LogisticsTrackModel {
	return &defaultLogisticsTrackModel{conn: conn}
}

func (m *defaultLogisticsTrackModel) Insert(ctx context.Context, data *LogisticsTrack) (*LogisticsTrack, error) {
	if data.Status == "" {
		data.Status = TrackStatusPending
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	if data.CreateTime == "" {
		data.CreateTime = now
	}
	data.UpdateTime = now
	if data.LastSyncTime == "" {
		data.LastSyncTime = now
	}
	tracesJSON := tracesToJSON(data.Traces)

	query := fmt.Sprintf(`INSERT INTO %s (tracking_no, express_code, express_name, biz_type, biz_no, status, traces, last_sync_time, create_time, update_time) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, logisticsTrackTable)
	result, err := m.conn.ExecCtx(ctx, query, data.TrackingNo, data.ExpressCode, data.ExpressName, data.BizType, data.BizNo, data.Status, tracesJSON, data.LastSyncTime, data.CreateTime, data.UpdateTime)
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

func (m *defaultLogisticsTrackModel) FindByTrackingNo(ctx context.Context, trackingNo string) (*LogisticsTrack, error) {
	var row logisticsTrackRow
	query := fmt.Sprintf(`SELECT id, tracking_no, express_code, express_name, biz_type, biz_no, status, traces, last_sync_time, create_time, update_time FROM %s WHERE tracking_no = ?`, logisticsTrackTable)
	err := m.conn.QueryRowCtx(ctx, &row, query, trackingNo)
	if err != nil {
		return nil, err
	}
	return rowToTrack(&row), nil
}

func (m *defaultLogisticsTrackModel) FindNonTerminal(ctx context.Context) ([]*LogisticsTrack, error) {
	query := fmt.Sprintf(`SELECT id, tracking_no, express_code, express_name, biz_type, biz_no, status, traces, last_sync_time, create_time, update_time FROM %s WHERE status != ?`, logisticsTrackTable)
	var rows []*logisticsTrackRow
	err := m.conn.QueryRowsCtx(ctx, &rows, query, TrackStatusSigned)
	if err != nil {
		return nil, err
	}
	list := make([]*LogisticsTrack, 0, len(rows))
	for _, row := range rows {
		list = append(list, rowToTrack(row))
	}
	return list, nil
}

func (m *defaultLogisticsTrackModel) Update(ctx context.Context, data *LogisticsTrack) error {
	tracesJSON := tracesToJSON(data.Traces)
	now := time.Now().Format("2006-01-02 15:04:05")
	data.UpdateTime = now
	if data.LastSyncTime == "" {
		data.LastSyncTime = now
	}
	query := fmt.Sprintf(`UPDATE %s SET express_code=?, express_name=?, status=?, traces=?, last_sync_time=?, update_time=? WHERE id=?`, logisticsTrackTable)
	_, err := m.conn.ExecCtx(ctx, query, data.ExpressCode, data.ExpressName, data.Status, tracesJSON, data.LastSyncTime, data.UpdateTime, data.Id)
	return err
}

// rowToTrack 将 DB 行转换为对外结构体
func rowToTrack(row *logisticsTrackRow) *LogisticsTrack {
	return &LogisticsTrack{
		Id:           row.Id,
		TrackingNo:   row.TrackingNo,
		ExpressCode:  row.ExpressCode,
		ExpressName:  row.ExpressName,
		BizType:      row.BizType,
		BizNo:        row.BizNo,
		Status:       row.Status,
		Traces:       jsonToTraces(row.Traces),
		LastSyncTime: row.LastSyncTime,
		CreateTime:   row.CreateTime,
		UpdateTime:   row.UpdateTime,
	}
}

func tracesToJSON(traces []TrackTrace) string {
	if len(traces) == 0 {
		return "[]"
	}
	b, err := json.Marshal(traces)
	if err != nil {
		return "[]"
	}
	return string(b)
}

func jsonToTraces(s string) []TrackTrace {
	if s == "" {
		return []TrackTrace{}
	}
	var traces []TrackTrace
	if err := json.Unmarshal([]byte(s), &traces); err != nil {
		return []TrackTrace{}
	}
	return traces
}

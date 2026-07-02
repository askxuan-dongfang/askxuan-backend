package model

import (
	"sync"
)

// 举报状态常量
const (
	ReportStatusPending  = "pending"  // 待处理
	ReportStatusHandled  = "handled"  // 已处理
	ReportStatusRejected = "rejected" // 已驳回
)

// 举报目标类型
const (
	ReportTargetDesign  = "design"
	ReportTargetComment = "comment"
	ReportTargetMaster  = "master"
	ReportTargetTemple  = "temple"
)

// reportTransitions 举报状态机合法流转
var reportTransitions = map[string]map[string]bool{
	ReportStatusPending: {
		ReportStatusHandled:  true,
		ReportStatusRejected: true,
	},
}

// CanTransitReport 校验举报状态流转是否合法
func CanTransitReport(from, to string) bool {
	if from == to {
		return false
	}
	allowed, ok := reportTransitions[from]
	if !ok {
		return false
	}
	return allowed[to]
}

// Report 举报结构体
type Report struct {
	Id           int64  `json:"id"`
	ReporterId   string `json:"reporterId"`
	TargetType   string `json:"targetType"`
	TargetId     string `json:"targetId"`
	Reason       string `json:"reason"`
	EvidenceUrls string `json:"evidenceUrls"`
	Status       string `json:"status"`
	HandlerId    string `json:"handlerId"`
	HandleResult string `json:"handleResult"`
	CreateTime   string `json:"createTime"`
}

// ---- 内存存储（MVP 阶段不连 DB）----

type reportStore struct {
	mu   sync.RWMutex
	list []Report
	seq  int64
}

var globalReportStore = &reportStore{
	list: []Report{
		{
			Id:           1,
			ReporterId:   "U003",
			TargetType:   ReportTargetDesign,
			TargetId:     "DD20260615001",
			Reason:       "设计涉及侵权",
			EvidenceUrls: `["https://oss.askxuan.com/report/1.jpg"]`,
			Status:       ReportStatusPending,
			CreateTime:   "2026-06-26 09:00:00",
		},
		{
			Id:           2,
			ReporterId:   "U001",
			TargetType:   ReportTargetComment,
			TargetId:     "RV20260610005",
			Reason:       "评论内容含不当言论",
			EvidenceUrls: `[]`,
			Status:       ReportStatusHandled,
			HandlerId:    "ADMIN001",
			HandleResult: "已删除违规评论",
			CreateTime:   "2026-06-22 14:00:00",
		},
	},
	seq: 2,
}

// ListReports 查询举报列表，支持按 targetType/status 筛选 + 分页
func ListReports(targetType, status string, page, size int) ([]Report, int64) {
	globalReportStore.mu.RLock()
	defer globalReportStore.mu.RUnlock()

	filtered := make([]Report, 0, len(globalReportStore.list))
	for _, rp := range globalReportStore.list {
		if targetType != "" && rp.TargetType != targetType {
			continue
		}
		if status != "" && rp.Status != status {
			continue
		}
		filtered = append(filtered, rp)
	}

	total := int64(len(filtered))
	start := (page - 1) * size
	if start < 0 {
		start = 0
	}
	if start > len(filtered) {
		start = len(filtered)
	}
	end := start + size
	if end > len(filtered) {
		end = len(filtered)
	}
	return filtered[start:end], total
}

// FindReportByID 按ID查询举报记录
func FindReportByID(id int64) (Report, bool) {
	globalReportStore.mu.RLock()
	defer globalReportStore.mu.RUnlock()
	for _, rp := range globalReportStore.list {
		if rp.Id == id {
			return rp, true
		}
	}
	return Report{}, false
}

// UpdateReport 更新举报记录，找到 id 则更新 status/handlerId/handleResult 并返回 true
func UpdateReport(id int64, status, handlerId, handleResult string) bool {
	globalReportStore.mu.Lock()
	defer globalReportStore.mu.Unlock()
	for i := range globalReportStore.list {
		if globalReportStore.list[i].Id == id {
			globalReportStore.list[i].Status = status
			globalReportStore.list[i].HandlerId = handlerId
			globalReportStore.list[i].HandleResult = handleResult
			return true
		}
	}
	return false
}

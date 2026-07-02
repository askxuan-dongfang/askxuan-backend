package model

import (
	"sync"
	"time"
)

// 举报状态常量
const (
	ReportStatusPending  = "pending"  // 待处理
	ReportStatusHandled  = "handled"  // 已处理
	ReportStatusRejected = "rejected" // 已驳回
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

// ReviewReport 评价举报结构体
type ReviewReport struct {
	Id           int64  `json:"id"`
	ReviewId     int64  `json:"reviewId"`
	ReporterId   string `json:"reporterId"`
	Reason       string `json:"reason"`
	Status       string `json:"status"`
	HandleResult string `json:"handleResult"`
	CreateTime   string `json:"createTime"`
}

// ---- 内存存储（MVP 阶段不连 DB）----

type reviewReportStore struct {
	mu   sync.RWMutex
	list []ReviewReport
	seq  int64
}

var globalReviewReportStore = &reviewReportStore{
	list: []ReviewReport{
		{
			Id:         1,
			ReviewId:   2,
			ReporterId: "T003",
			Reason:     "评价内容涉及不实信息",
			Status:     ReportStatusPending,
			CreateTime: "2026-06-26 09:00:00",
		},
	},
	seq: 1,
}

// ListReports 查询举报列表，支持按 status 筛选 + 分页
func ListReports(status string, page, size int) ([]ReviewReport, int64) {
	globalReviewReportStore.mu.RLock()
	defer globalReviewReportStore.mu.RUnlock()

	filtered := make([]ReviewReport, 0, len(globalReviewReportStore.list))
	for _, rp := range globalReviewReportStore.list {
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

// CreateReport 新建举报，seq 自增，默认 status=pending，设置 createTime
func CreateReport(r ReviewReport) ReviewReport {
	globalReviewReportStore.mu.Lock()
	defer globalReviewReportStore.mu.Unlock()

	globalReviewReportStore.seq++
	r.Id = globalReviewReportStore.seq
	if r.Status == "" {
		r.Status = ReportStatusPending
	}
	r.CreateTime = time.Now().Format("2006-01-02 15:04:05")
	globalReviewReportStore.list = append(globalReviewReportStore.list, r)
	return r
}

// FindReportByID 按ID查询举报
func FindReportByID(id int64) (ReviewReport, bool) {
	globalReviewReportStore.mu.RLock()
	defer globalReviewReportStore.mu.RUnlock()

	for _, r := range globalReviewReportStore.list {
		if r.Id == id {
			return r, true
		}
	}
	return ReviewReport{}, false
}

// UpdateReportStatus 更新举报状态和处理结果，找到并更新返回 true，未找到返回 false
func UpdateReportStatus(id int64, status, handleResult string) bool {
	globalReviewReportStore.mu.Lock()
	defer globalReviewReportStore.mu.Unlock()

	for i := range globalReviewReportStore.list {
		if globalReviewReportStore.list[i].Id == id {
			globalReviewReportStore.list[i].Status = status
			globalReviewReportStore.list[i].HandleResult = handleResult
			return true
		}
	}
	return false
}

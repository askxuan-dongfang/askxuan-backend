package model

import (
	"sync"
)

// 审核状态常量（参照 state-machines.md 第7节）
const (
	AuditStatusPending   = "pending"    // 待审核
	AuditStatusApproved  = "approved"   // 审核通过（设计）
	AuditStatusRejected  = "rejected"   // 审核驳回
	AuditStatusFirstPass = "first_pass" // 初审通过（寺院）
	AuditStatusFinalPass = "final_pass" // 终审通过（寺院）
	AuditStatusVerified  = "verified"   // 已认证（法师）
)

// 业务类型
const (
	BizTypeDesign  = "design"
	BizTypeTemple  = "temple"
	BizTypeMaster  = "master"
	BizTypeComment = "comment"
)

// auditTransitions 审核状态机合法流转（综合设计/寺院/法师三种）
// 参照 state-machines.md 7.1/7.2/7.3
var auditTransitions = map[string]map[string]bool{
	AuditStatusPending: {
		AuditStatusApproved:  true, // 设计通过
		AuditStatusRejected:  true, // 驳回
		AuditStatusFirstPass: true, // 寺院初审通过
		AuditStatusVerified:  true, // 法师认证通过
	},
	AuditStatusFirstPass: {
		AuditStatusFinalPass: true, // 寺院终审通过
		AuditStatusRejected:  true, // 终审驳回
	},
	AuditStatusRejected: {
		AuditStatusPending: true, // 修改后重新提交
	},
}

// CanTransitAudit 校验审核状态流转是否合法
func CanTransitAudit(from, to string) bool {
	if from == to {
		return false
	}
	allowed, ok := auditTransitions[from]
	if !ok {
		return false
	}
	return allowed[to]
}

// IsAuditTerminalStatus 是否终态
func IsAuditTerminalStatus(s string) bool {
	return s == AuditStatusApproved || s == AuditStatusFinalPass || s == AuditStatusVerified
}

// AuditQueue 审核队列结构体
type AuditQueue struct {
	Id              int64  `json:"id"`
	BizType         string `json:"bizType"`
	BizId           string `json:"bizId"`
	SubmitterId     string `json:"submitterId"`
	ContentSnapshot string `json:"contentSnapshot"`
	Status          string `json:"status"`
	AuditorId       string `json:"auditorId"`
	AuditTime       string `json:"auditTime"`
	AuditRemark     string `json:"auditRemark"`
	CreateTime      string `json:"createTime"`
}

// ---- 内存存储（MVP 阶段不连 DB）----

type auditQueueStore struct {
	mu   sync.RWMutex
	list []AuditQueue
	seq  int64
}

var globalAuditQueueStore = &auditQueueStore{
	list: []AuditQueue{
		{
			Id:              1,
			BizType:         BizTypeDesign,
			BizId:           "DD20260628001",
			SubmitterId:     "U001",
			ContentSnapshot: `{"name":"紫檀开光手串","materials":["小叶紫檀","蜜蜡佛头"]}`,
			Status:          AuditStatusPending,
			CreateTime:      "2026-06-28 14:00:00",
		},
		{
			Id:              2,
			BizType:         BizTypeTemple,
			BizId:           "T005",
			SubmitterId:     "T005",
			ContentSnapshot: `{"name":"普陀山","type":"汉传佛教","status":"待审核"}`,
			Status:          AuditStatusPending,
			CreateTime:      "2026-06-29 10:00:00",
		},
		{
			Id:              3,
			BizType:         BizTypeMaster,
			BizId:           "M005",
			SubmitterId:     "T005",
			ContentSnapshot: `{"name":"慧明法师","credential":"戒牒编号XXX"}`,
			Status:          AuditStatusPending,
			CreateTime:      "2026-06-29 11:00:00",
		},
		{
			Id:              4,
			BizType:         BizTypeDesign,
			BizId:           "DD20260620002",
			SubmitterId:     "U002",
			ContentSnapshot: `{"name":"星月菩提手串","materials":["星月菩提","白水晶隔片"]}`,
			Status:          AuditStatusApproved,
			AuditorId:       "ADMIN001",
			AuditTime:       "2026-06-20 16:00:00",
			CreateTime:      "2026-06-20 15:00:00",
		},
	},
	seq: 4,
}

// ListAuditQueue 查询审核队列，支持按 bizType/status 筛选 + 分页
func ListAuditQueue(bizType, status string, page, size int) ([]AuditQueue, int64) {
	globalAuditQueueStore.mu.RLock()
	defer globalAuditQueueStore.mu.RUnlock()

	filtered := make([]AuditQueue, 0, len(globalAuditQueueStore.list))
	for _, a := range globalAuditQueueStore.list {
		if bizType != "" && a.BizType != bizType {
			continue
		}
		if status != "" && a.Status != status {
			continue
		}
		filtered = append(filtered, a)
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

// FindAuditQueueByID 按ID查询审核记录
func FindAuditQueueByID(id int64) (AuditQueue, bool) {
	globalAuditQueueStore.mu.RLock()
	defer globalAuditQueueStore.mu.RUnlock()
	for _, a := range globalAuditQueueStore.list {
		if a.Id == id {
			return a, true
		}
	}
	return AuditQueue{}, false
}

// UpdateAuditQueueStatus 更新审核记录状态，找到 id 则更新 status/auditorId/auditTime/auditRemark 并返回 true
func UpdateAuditQueueStatus(id int64, status, auditorId, auditTime, remark string) bool {
	globalAuditQueueStore.mu.Lock()
	defer globalAuditQueueStore.mu.Unlock()
	for i := range globalAuditQueueStore.list {
		if globalAuditQueueStore.list[i].Id == id {
			globalAuditQueueStore.list[i].Status = status
			globalAuditQueueStore.list[i].AuditorId = auditorId
			globalAuditQueueStore.list[i].AuditTime = auditTime
			globalAuditQueueStore.list[i].AuditRemark = remark
			return true
		}
	}
	return false
}

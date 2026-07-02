package types

// AuditQueue 审核队列
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

type AuditQueueListReq struct {
	BizType string `form:"bizType,optional"`
	Status  string `form:"status,optional"`
	Page    int    `form:"page,default=1"`
	Size    int    `form:"size,default=20"`
}

type AuditQueueListResp struct {
	Total int64       `json:"total"`
	List  []AuditQueue `json:"list"`
	Page  int         `json:"page"`
	Size  int         `json:"size"`
}

type AuditQueueDetailReq struct {
	Id int64 `path:"id"`
}

type AuditApproveReq struct {
	Id        int64  `path:"id"`
	AuditorId string `json:"auditorId"`
	Remark    string `json:"remark,optional"`
}

type AuditApproveResp struct {
	Id     int64  `json:"id"`
	Status string `json:"status"`
}

type AuditRejectReq struct {
	Id        int64  `path:"id"`
	AuditorId string `json:"auditorId"`
	Remark    string `json:"remark"`
}

type AuditRejectResp struct {
	Id     int64  `json:"id"`
	Status string `json:"status"`
}

// Report 举报
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

type ReportListReq struct {
	TargetType string `form:"targetType,optional"`
	Status     string `form:"status,optional"`
	Page       int    `form:"page,default=1"`
	Size       int    `form:"size,default=20"`
}

type ReportListResp struct {
	Total int64    `json:"total"`
	List  []Report `json:"list"`
	Page  int      `json:"page"`
	Size  int      `json:"size"`
}

type ReportHandleReq struct {
	Id           int64  `path:"id"`
	HandlerId    string `json:"handlerId"`
	HandleResult string `json:"handleResult"`
	Remark       string `json:"remark,optional"`
}

type ReportHandleResp struct {
	Id     int64  `json:"id"`
	Status string `json:"status"`
}

// SensitiveWord 敏感词
type SensitiveWord struct {
	Id         int64  `json:"id"`
	Word       string `json:"word"`
	Category   string `json:"category"`
	Status     string `json:"status"`
	CreateTime string `json:"createTime"`
}

type SensitiveWordListReq struct {
	Category string `form:"category,optional"`
	Status   string `form:"status,optional"`
	Keyword  string `form:"keyword,optional"`
	Page     int    `form:"page,default=1"`
	Size     int    `form:"size,default=20"`
}

type SensitiveWordListResp struct {
	Total int64          `json:"total"`
	List  []SensitiveWord `json:"list"`
	Page  int            `json:"page"`
	Size  int            `json:"size"`
}

type SensitiveWordCreateReq struct {
	Word     string `json:"word"`
	Category string `json:"category"`
}

type SensitiveWordCreateResp struct {
	Id int64 `json:"id"`
}

type SensitiveWordDeleteReq struct {
	Id int64 `path:"id"`
}

// 审核统计
type StatisticsReq struct {
	BizType string `form:"bizType,optional"`
}

type StatisticsResp struct {
	TotalCount    int64   `json:"totalCount"`
	PendingCount  int64   `json:"pendingCount"`
	ApprovedCount int64   `json:"approvedCount"`
	RejectedCount int64   `json:"rejectedCount"`
	PassRate      float64 `json:"passRate"`
	AvgAuditTime  int64   `json:"avgAuditTime"`
}

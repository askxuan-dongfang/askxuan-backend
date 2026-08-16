package types

// ============ 法师基础 ============

// Master 法师
type Master struct {
	Id                     string   `json:"id"`
	DharmaName             string   `json:"dharmaName"`
	LayName                string   `json:"layName"`
	TempleId               string   `json:"templeId"`
	TempleName             string   `json:"templeName"`
	Position               string   `json:"position"`
	BeliefCode             string   `json:"beliefCode"`
	Sect                   string   `json:"sect"`
	Type                   string   `json:"type"`
	AuthStatus             string   `json:"authStatus"`
	ShelfStatus            string   `json:"shelfStatus"`
	PlatformStatus         string   `json:"platformStatus"`
	ManageBy               string   `json:"manageBy"` // temple=寺庙绑定 / platform=平台(野生)
	ServiceTags            []MasterServiceTagItem `json:"serviceTags,omitempty"` // 大师服务标签
	Specialties            []string `json:"specialties"`
	Avatar                 string   `json:"avatar"`
	Rating                 float64  `json:"rating"`
	ConsultEnabled         bool     `json:"consultEnabled"`
	ConsultFee             float64  `json:"consultFee"`
	ConsultValidHours      int      `json:"consultValidHours"`
	ConsultResponseMinutes int      `json:"consultResponseMinutes"`
}

// ListReq 列表查询请求
type ListReq struct {
	BeliefCode  string `form:"beliefCode,optional"`
	Sect        string `form:"sect,optional"`
	Type        string `form:"type,optional"`
	TempleId    string `form:"templeId,optional"`
	ManageBy    string `form:"manageBy,optional"`    // temple/platform：找师傅专栏传 platform
	ServiceCode string `form:"serviceCode,optional"` // 按可提供服务筛选（master_service_tag，enabled）
	Page        int    `form:"page,default=1"`
	Size        int    `form:"size,default=20"`
}

// ListResp 列表查询响应
type ListResp struct {
	Total int64    `json:"total"`
	List  []Master `json:"list"`
	Page  int      `json:"page"`
	Size  int      `json:"size"`
}

// PlatformMasterListReq 平台法师全量列表请求。
type PlatformMasterListReq struct {
	BeliefCode     string `form:"beliefCode,optional"`
	Sect           string `form:"sect,optional"`
	Type           string `form:"type,optional"`
	TempleId       string `form:"templeId,optional"`
	AuthStatus     string `form:"authStatus,optional"`
	ShelfStatus    string `form:"shelfStatus,optional"`
	PlatformStatus string `form:"platformStatus,optional"`
	Page           int    `form:"page,default=1"`
	Size           int    `form:"size,default=20"`
}

type MasterConsultConfigReq struct {
	Id                     string  `path:"id"`
	ConsultEnabled         bool    `json:"consultEnabled"`
	ConsultFee             float64 `json:"consultFee"`
	ConsultValidHours      int     `json:"consultValidHours"`
	ConsultResponseMinutes int     `json:"consultResponseMinutes"`
}

// DetailReq 详情请求
type DetailReq struct {
	Id string `path:"id"`
}

// ============ 寺院管理台 - 法师管理 ============

// AdminMasterListReq 管理台法师列表请求
type AdminMasterListReq struct {
	TempleId string `form:"templeId,optional"` // 服务端隔离以网关 X-Temple-Code 为准，客户端参数仅供兼容
	Status   string `form:"status,optional"`
	Page     int    `form:"page,default=1"`
	Size     int    `form:"size,default=20"`
}

// AdminMasterListResp 管理台法师列表响应
type AdminMasterListResp struct {
	Total int64    `json:"total"`
	List  []Master `json:"list"`
	Page  int      `json:"page"`
	Size  int      `json:"size"`
}

// AdminMasterCreateReq 创建法师请求
type AdminMasterCreateReq struct {
	DharmaName             string   `json:"dharmaName"`
	LayName                string   `json:"layName"`
	TempleId               string   `json:"templeId"`
	TempleName             string   `json:"templeName,optional"`
	Position               string   `json:"position"`
	BeliefCode             string   `json:"beliefCode"`
	Sect                   string   `json:"sect"`
	Type                   string   `json:"type"`
	Specialties            []string `json:"specialties"`
	Avatar                 string   `json:"avatar,optional"`
	ConsultEnabled         bool     `json:"consultEnabled,optional"`
	ConsultFee             float64  `json:"consultFee,optional"`
	ConsultValidHours      int      `json:"consultValidHours,optional"`
	ConsultResponseMinutes int      `json:"consultResponseMinutes,optional"`
}

// AdminMasterCreateResp 创建法师响应
type AdminMasterCreateResp struct {
	Id string `json:"id"`
}

// AdminMasterDetailReq 寺庙台法师详情请求
type AdminMasterDetailReq struct {
	Id string `path:"id"` // 法师编码 code
}

// AdminMasterUpdateReq 更新法师请求
type AdminMasterUpdateReq struct {
	Id                     string   `path:"id"`
	DharmaName             string   `json:"dharmaName,optional"`
	LayName                string   `json:"layName,optional"`
	Position               string   `json:"position,optional"`
	BeliefCode             string   `json:"beliefCode,optional"`
	Sect                   string   `json:"sect,optional"`
	Specialties            []string `json:"specialties,optional"`
	Avatar                 string   `json:"avatar,optional"`
	ConsultEnabled         bool     `json:"consultEnabled,optional"`
	ConsultFee             float64  `json:"consultFee,optional"`
	ConsultValidHours      int      `json:"consultValidHours,optional"`
	ConsultResponseMinutes int      `json:"consultResponseMinutes,optional"`
}

// AdminMasterStatusReq 法师上下架请求
type AdminMasterStatusReq struct {
	Id     string `path:"id"`
	Status string `json:"status"`
}

// AdminMasterStatusResp 法师上下架响应
type AdminMasterStatusResp struct {
	Id     string `json:"id"`
	Status string `json:"status"`
}

// PlatformMasterDetailReq 平台法师详情请求
type PlatformMasterDetailReq struct {
	Id string `path:"id"` // 法师编码 code
}

// PlatformMasterUpdateReq 平台编辑法师请求（野生大师无寺庙归属，平台为唯一管理方）
type PlatformMasterUpdateReq struct {
	Id                     string   `path:"id"`
	DharmaName             string   `json:"dharmaName,optional"`
	LayName                string   `json:"layName,optional"`
	Position               string   `json:"position,optional"`
	BeliefCode             string   `json:"beliefCode,optional"`
	Sect                   string   `json:"sect,optional"`
	Type                   string   `json:"type,optional"`
	Specialties            []string `json:"specialties,optional"`
	Avatar                 string   `json:"avatar,optional"`
	ConsultEnabled         bool     `json:"consultEnabled,optional"`
	ConsultFee             float64  `json:"consultFee,optional"`
	ConsultValidHours      int      `json:"consultValidHours,optional"`
	ConsultResponseMinutes int      `json:"consultResponseMinutes,optional"`
}

// ============ 法师工作台 - 加持任务（修复 Gap-4/15） ============

// BlessingTask 加持任务
type BlessingTask struct {
	Id              int64    `json:"id"`
	TaskNo          string   `json:"taskNo"`
	DiyOrderNo      string   `json:"diyOrderNo"`
	TempleCode      string   `json:"templeCode"`
	MasterCode      string   `json:"masterCode"`
	Status          string   `json:"status"`
	CertificateUrls []string `json:"certificateUrls"`
	AssignTime      string   `json:"assignTime"`
	CompleteTime    string   `json:"completeTime"`
	CreateTime      string   `json:"createTime"`
}

// BlessingTaskListReq 加持任务列表请求
type BlessingTaskListReq struct {
	Status string `form:"status,optional"`
	Page   int    `form:"page,default=1"`
	Size   int    `form:"size,default=20"`
}

// BlessingTaskListResp 加持任务列表响应
type BlessingTaskListResp struct {
	Total int64          `json:"total"`
	List  []BlessingTask `json:"list"`
	Page  int            `json:"page"`
	Size  int            `json:"size"`
}

// BlessingTaskDetailReq 加持任务详情请求
type BlessingTaskDetailReq struct {
	Id int64 `path:"id"`
}

// BlessingTaskActionReq 加持任务操作请求（accept/start/reject）
type BlessingTaskActionReq struct {
	Id int64 `path:"id"`
}

// BlessingTaskCompleteReq 完成加持请求
type BlessingTaskCompleteReq struct {
	Id              int64    `path:"id"`
	CertificateUrls []string `json:"certificateUrls"`
}

// ============ 法师工作台 - 日程管理 ============

// MasterSchedule 法师日程
type MasterSchedule struct {
	Id         int64    `json:"id"`
	MasterCode string   `json:"masterCode"`
	Date       string   `json:"date"`
	TimeSlots  []string `json:"timeSlots"`
	Status     string   `json:"status"`
	CreateTime string   `json:"createTime"`
}

// ScheduleListReq 日程列表请求
type ScheduleListReq struct {
	Date string `form:"date,optional"`
	Page int    `form:"page,default=1"`
	Size int    `form:"size,default=20"`
}

// ScheduleListResp 日程列表响应
type ScheduleListResp struct {
	Total int64            `json:"total"`
	List  []MasterSchedule `json:"list"`
	Page  int              `json:"page"`
	Size  int              `json:"size"`
}

// ScheduleUpdateReq 更新日程请求
type ScheduleUpdateReq struct {
	Date      string   `json:"date"`
	TimeSlots []string `json:"timeSlots"`
	Status    string   `json:"status"`
}

// ScheduleUpdateResp 更新日程响应
type ScheduleUpdateResp struct {
	Id int64 `json:"id"`
}

// ============ 法师工作台 - 收益 ============

// EarningsSummaryReq 收益汇总请求
type EarningsSummaryReq struct {
}

// EarningsTrendItem 收益趋势项
type EarningsTrendItem struct {
	Month  string  `json:"month"`
	Amount float64 `json:"amount"`
}

// EarningsSummaryResp 收益汇总响应
type EarningsSummaryResp struct {
	MonthIncome  float64             `json:"monthIncome"`
	TotalIncome  float64             `json:"totalIncome"`
	Withdrawable float64             `json:"withdrawable"`
	Trend        []EarningsTrendItem `json:"trend"`
}

// EarningsDetailReq 收益明细请求
type EarningsDetailReq struct {
	ServiceType string `form:"serviceType,optional"`
	Page        int    `form:"page,default=1"`
	Size        int    `form:"size,default=20"`
}

// EarningsDetailItem 收益明细项
type EarningsDetailItem struct {
	Id           int64   `json:"id"`
	Date         string  `json:"date"`
	ServiceType  string  `json:"serviceType"`
	UserName     string  `json:"userName"`
	Amount       float64 `json:"amount"`
	SettleStatus string  `json:"settleStatus"`
}

// EarningsDetailResp 收益明细响应
type EarningsDetailResp struct {
	Total int64                `json:"total"`
	List  []EarningsDetailItem `json:"list"`
	Page  int                  `json:"page"`
	Size  int                  `json:"size"`
}

// ============ 法师工作台 - 个人资料 ============

// MasterProfileReq 法师资料请求
type MasterProfileReq struct {
}

// MasterProfileResp 法师资料响应
type MasterProfileResp struct {
	Id          string   `json:"id"`
	DharmaName  string   `json:"dharmaName"`
	LayName     string   `json:"layName"`
	TempleId    string   `json:"templeId"`
	Position    string   `json:"position"`
	Sect        string   `json:"sect"`
	Type        string   `json:"type"`
	AuthStatus  string   `json:"authStatus"`
	Specialties []string `json:"specialties"`
	Avatar      string   `json:"avatar"`
	Bio         string   `json:"bio"`
	Pricing     string   `json:"pricing"`
	Rating      float64  `json:"rating"`
	ManageBy    string                    `json:"manageBy"`               // temple/platform
	ServiceTags []MasterServiceTagItemResp `json:"serviceTags,omitempty"` // 我的服务标签
}

// MasterProfileUpdateReq 法师资料更新请求
type MasterProfileUpdateReq struct {
	Bio         string   `json:"bio,optional"`
	Specialties []string `json:"specialties,optional"`
	Avatar      string   `json:"avatar,optional"`
	Pricing     string   `json:"pricing,optional"`
}

// ============ 平台审核 ============

// MasterAudit 法师资质审核
type MasterAudit struct {
	Id             int64    `json:"id"`
	MasterCode     string   `json:"masterCode"`
	TempleCode     string   `json:"templeCode"`
	CredentialUrls []string `json:"credentialUrls"`
	Status         string   `json:"status"`
	AuditorId      int64    `json:"auditorId"`
	AuditRemark    string   `json:"auditRemark"`
	CreateTime     string   `json:"createTime"`
}

// MasterAuditListReq 审核列表请求
type MasterAuditListReq struct {
	Status string `form:"status,optional"`
	Page   int    `form:"page,default=1"`
	Size   int    `form:"size,default=20"`
}

// MasterAuditListResp 审核列表响应
type MasterAuditListResp struct {
	Total int64         `json:"total"`
	List  []MasterAudit `json:"list"`
	Page  int           `json:"page"`
	Size  int           `json:"size"`
}

// MasterAuditActionReq 审核操作请求
type MasterAuditActionReq struct {
	Id          int64  `path:"id"`
	AuditRemark string `json:"auditRemark,optional"`
}

// MasterServiceTagItem 大师服务标签项
type MasterServiceTagItem struct {
	ServiceCode string  `json:"serviceCode"`
	Price       float64 `json:"price"`
	Status      string  `json:"status,optional"` // enabled/disabled/pending_review（go-zero 请求解析仅认 optional）
}

// MasterServiceTagItemResp 标签项（含状态）
type MasterServiceTagItemResp struct {
	ServiceCode string  `json:"serviceCode"`
	Price       float64 `json:"price"`
	Status      string  `json:"status"`
}

// MasterServiceTagsReq 更新大师服务标签请求
type MasterServiceTagsReq struct {
	Tags []MasterServiceTagItem `json:"tags"`
}

// PlatformMasterTagsUpdateReq 平台配置大师服务标签请求（path 为法师编码）
type PlatformMasterTagsUpdateReq struct {
	Id   string                 `path:"id"`
	Tags []MasterServiceTagItem `json:"tags"`
}

// MasterServiceTagsResp 大师服务标签列表
type MasterServiceTagsResp struct {
	List []MasterServiceTagItemResp `json:"list"`
}

// PlatformMasterCreateReq 平台创建野生大师（无寺庙，需资质审核后上架）
type PlatformMasterCreateReq struct {
	DharmaName             string   `json:"dharmaName"`
	LayName                string   `json:"layName,optional"`
	Position               string   `json:"position,optional"`
	BeliefCode             string   `json:"beliefCode"`
	Sect                   string   `json:"sect"`
	Type                   string   `json:"type"`
	Specialties            []string `json:"specialties,optional"`
	Avatar                 string   `json:"avatar,optional"`
	ConsultEnabled         bool     `json:"consultEnabled,optional"`
	ConsultFee             float64  `json:"consultFee,optional"`
	ConsultValidHours      int      `json:"consultValidHours,optional"`
	ConsultResponseMinutes int      `json:"consultResponseMinutes,optional"`
}

// PlatformMasterCreateResp 平台创建野生大师响应
type PlatformMasterCreateResp struct {
	Id string `json:"id"`
}

// MasterPlatformStatusReq 平台法师状态请求
type MasterPlatformStatusReq struct {
	Id     string `path:"id"`
	Status string `json:"status"`
}

// MasterPlatformStatusResp 平台法师状态响应
type MasterPlatformStatusResp struct {
	Id     string `json:"id"`
	Status string `json:"status"`
}

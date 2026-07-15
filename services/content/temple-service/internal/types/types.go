package types

type BeliefProfile struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
	CoverImage  string `json:"coverImage"`
	Sort        int    `json:"sort"`
}

type BeliefReq struct {
	Code string `path:"code"`
}
type BeliefUpdateReq struct {
	Code        string `path:"code"`
	Name        string `json:"name"`
	Summary     string `json:"summary,optional"`
	Description string `json:"description"`
	CoverImage  string `json:"coverImage,optional"`
	Sort        int    `json:"sort,optional"`
}

// ============ 寺院基础 ============

// Temple 寺院
type Temple struct {
	Id           string   `json:"id"`
	Name         string   `json:"name"`
	Region       string   `json:"region"`
	Type         string   `json:"type"`
	BeliefCode   string   `json:"beliefCode"`
	Sect         string   `json:"sect"`
	Status       string   `json:"status"`
	Address      string   `json:"address"`
	CoverImage   string   `json:"coverImage"`
	Rating       float64  `json:"rating"`
	Description  string   `json:"description"`
	ServiceCodes []string `json:"serviceCodes"`
	ServiceTags  []string `json:"serviceTags"`
	ServiceCount int      `json:"serviceCount"`
}

// TempleDetail 寺院详情（含图片+服务）
type TempleDetail struct {
	Temple   Temple          `json:"temple"`
	Images   []TempleImage   `json:"images"`
	Services []TempleService `json:"services"`
}

// ListReq 列表查询请求
type ListReq struct {
	BeliefCode  string `form:"beliefCode,optional"`
	Sect        string `form:"sect,optional"`
	Type        string `form:"type,optional"`
	Region      string `form:"region,optional"`
	ServiceCode string `form:"serviceCode,optional"`
	Page        int    `form:"page,default=1"`
	Size        int    `form:"size,default=20"`
}

// ListResp 列表查询响应
type ListResp struct {
	Total int64    `json:"total"`
	List  []Temple `json:"list"`
	Page  int      `json:"page"`
	Size  int      `json:"size"`
}

// DetailReq 详情请求
// 注：Id 加 optional 以兼容管理台 /info 接口（路径无 :id 参数）
type DetailReq struct {
	Id string `path:"id,optional"`
}

// ============ 寺院图片 ============

// TempleImage 寺院图片
type TempleImage struct {
	Id         int64  `json:"id"`
	TempleCode string `json:"templeCode"`
	Url        string `json:"url"`
	Type       string `json:"type"`
	Sort       int    `json:"sort"`
	CreateTime string `json:"createTime"`
}

// TempleImageCreateReq 上传图片请求
type TempleImageCreateReq struct {
	Url  string `json:"url"`
	Type string `json:"type"`
	Sort int    `json:"sort,optional"`
}

// TempleImageCreateResp 上传图片响应
type TempleImageCreateResp struct {
	Id int64 `json:"id"`
}

// TempleImageDeleteReq 删除图片请求
type TempleImageDeleteReq struct {
	Id int64 `path:"id"`
}

// ============ 寺院服务 ============

// TempleService 寺院自定义服务
type TempleService struct {
	Id          int64               `json:"id"`
	TempleCode  string              `json:"templeCode"`
	ServiceCode string              `json:"serviceCode"`
	ServiceName string              `json:"serviceName"`
	Price       float64             `json:"price"`
	TimeSlots   []string            `json:"timeSlots"`
	Slots       []TempleServiceSlot `json:"slots"`
	IntentTags  []string            `json:"intentTags"`
	Status      string              `json:"status"`
	CreateTime  string              `json:"createTime"`
}

type TempleServiceSlot struct {
	Code      string `json:"code"`
	Label     string `json:"label"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	Capacity  int    `json:"capacity"`
	Status    string `json:"status"`
	Sort      int    `json:"sort"`
}

// TempleServiceListReq 服务列表请求（C端按寺院ID查询）
type TempleServiceListReq struct {
	Id string `path:"id"`
}

// TempleServiceListResp 服务列表响应
type TempleServiceListResp struct {
	List []TempleService `json:"list"`
}

// TempleServiceCreateReq 创建服务请求
type TempleServiceCreateReq struct {
	ServiceCode string              `json:"serviceCode"`
	ServiceName string              `json:"serviceName"`
	Price       float64             `json:"price"`
	TimeSlots   []string            `json:"timeSlots"`
	Slots       []TempleServiceSlot `json:"slots,optional"`
	IntentTags  []string            `json:"intentTags,optional"`
}

// TempleServiceCreateResp 创建服务响应
type TempleServiceCreateResp struct {
	Id int64 `json:"id"`
}

// TempleServiceUpdateReq 更新服务请求
type TempleServiceUpdateReq struct {
	Id          int64               `path:"id"`
	ServiceName string              `json:"serviceName,optional"`
	Price       float64             `json:"price,optional"`
	TimeSlots   []string            `json:"timeSlots,optional"`
	Slots       []TempleServiceSlot `json:"slots,optional"`
	IntentTags  []string            `json:"intentTags,optional"`
}

// TempleServiceStatusReq 服务上下架请求
type TempleServiceStatusReq struct {
	Id     int64  `path:"id"`
	Status string `json:"status"`
}

// TempleServiceStatusResp 服务上下架响应
type TempleServiceStatusResp struct {
	Id     int64  `json:"id"`
	Status string `json:"status"`
}

// ============ 寺院信息编辑 ============

// TempleUpdateReq 编辑寺院信息请求
type TempleUpdateReq struct {
	Id          string `json:"id"`
	Name        string `json:"name,optional"`
	Region      string `json:"region,optional"`
	Type        string `json:"type,optional"`
	BeliefCode  string `json:"beliefCode,optional"`
	Sect        string `json:"sect,optional"`
	Address     string `json:"address,optional"`
	CoverImage  string `json:"coverImage,optional"`
	Description string `json:"description,optional"`
}

// ============ 加持任务（修复 Gap-3） ============

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

// BlessingAssignReq 分配法师请求
type BlessingAssignReq struct {
	Id         int64  `path:"id"`
	MasterCode string `json:"masterCode"`
}

// ============ 入驻审核 ============

// TempleAudit 寺院入驻审核
type TempleAudit struct {
	Id            int64    `json:"id"`
	TempleCode    string   `json:"templeCode"`
	ApplicantName string   `json:"applicantName"`
	ContactPhone  string   `json:"contactPhone"`
	CertUrls      []string `json:"certUrls"`
	Status        string   `json:"status"`
	AuditorId     int64    `json:"auditorId"`
	AuditRemark   string   `json:"auditRemark"`
	CreateTime    string   `json:"createTime"`
}

// TempleApplyReq 入驻申请请求
type TempleApplyReq struct {
	TempleCode    string   `json:"templeCode"`
	ApplicantName string   `json:"applicantName"`
	ContactPhone  string   `json:"contactPhone"`
	CertUrls      []string `json:"certUrls"`
}

// TempleApplyResp 入驻申请响应
type TempleApplyResp struct {
	Id int64 `json:"id"`
}

// TempleAuditListReq 审核列表请求
type TempleAuditListReq struct {
	Status string `form:"status,optional"`
	Page   int    `form:"page,default=1"`
	Size   int    `form:"size,default=20"`
}

// TempleAuditListResp 审核列表响应
type TempleAuditListResp struct {
	Total int64         `json:"total"`
	List  []TempleAudit `json:"list"`
	Page  int           `json:"page"`
	Size  int           `json:"size"`
}

// TempleAuditActionReq 审核操作请求
type TempleAuditActionReq struct {
	Id          int64  `path:"id"`
	AuditRemark string `json:"auditRemark,optional"`
}

// ============ 平台寺院状态 ============

// TemplePlatformStatusReq 平台寺院状态请求
type TemplePlatformStatusReq struct {
	Id     string `path:"id"`
	Status string `json:"status"`
}

// TemplePlatformStatusResp 平台寺院状态响应
type TemplePlatformStatusResp struct {
	Id     string `json:"id"`
	Status string `json:"status"`
}

// ============ 寺院报表（寺院管理台） ============

// TempleReportReq 寺院报表请求
type TempleReportReq struct {
	StartTime string `form:"startTime,optional"`
	EndTime   string `form:"endTime,optional"`
}

// TempleReportResp 寺院报表响应
type TempleReportResp struct {
	BookingTrend        []BookingTrendPoint `json:"bookingTrend"`
	RevenueStats        TempleRevenueStats  `json:"revenueStats"`
	ServiceDistribution []ServiceDistItem   `json:"serviceDistribution"`
	MasterRanking       []MasterRankItem    `json:"masterRanking"`
}

// BookingTrendPoint 预约趋势点
type BookingTrendPoint struct {
	Date     string  `json:"date"`
	Bookings int     `json:"bookings"`
	Revenue  float64 `json:"revenue"`
}

// TempleRevenueStats 寺院收入统计
type TempleRevenueStats struct {
	TotalRevenue    float64 `json:"totalRevenue"`
	BookingCount    int     `json:"bookingCount"`
	AvgBookingValue float64 `json:"avgBookingValue"`
	CompletedCount  int     `json:"completedCount"`
}

// ServiceDistItem 服务分布项
type ServiceDistItem struct {
	ServiceName string  `json:"serviceName"`
	Count       int     `json:"count"`
	Percentage  float64 `json:"percentage"`
}

// MasterRankItem 法师排名项
type MasterRankItem struct {
	MasterCode   string  `json:"masterCode"`
	MasterName   string  `json:"masterName"`
	BookingCount int     `json:"bookingCount"`
	Revenue      float64 `json:"revenue"`
}

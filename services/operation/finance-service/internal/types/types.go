package types

// Settlement 结算单
type Settlement struct {
	Id               int64   `json:"id"`
	SettlementNo     string  `json:"settlementNo"`
	SettleType       string  `json:"settleType"`
	TargetId         string  `json:"targetId"`
	TargetName       string  `json:"targetName"`
	PeriodStart      string  `json:"periodStart"`
	PeriodEnd        string  `json:"periodEnd"`
	OrderCount       int     `json:"orderCount"`
	TotalAmount      float64 `json:"totalAmount"`
	CommissionRate   float64 `json:"commissionRate"`
	CommissionAmount float64 `json:"commissionAmount"`
	SettleAmount     float64 `json:"settleAmount"`
	Status           string  `json:"status"`
	SourceType       string  `json:"sourceType"`
	SourceNo         string  `json:"sourceNo"`
	CreateTime       string  `json:"createTime"`
}

type SettlementListReq struct {
	SettleType string `form:"settleType,optional"`
	Status     string `form:"status,optional"`
	Page       int    `form:"page,default=1"`
	Size       int    `form:"size,default=20"`
}

type SettlementListResp struct {
	Total int64        `json:"total"`
	List  []Settlement `json:"list"`
	Page  int          `json:"page"`
	Size  int          `json:"size"`
}

type SettlementDetailReq struct {
	Id int64 `path:"id"`
}

type SettlementConfirmReq struct {
	Id int64 `path:"id"`
}

type SettlementConfirmResp struct {
	Id     int64  `json:"id"`
	Status string `json:"status"`
}

// Withdrawal 提现申请
type Withdrawal struct {
	Id            int64   `json:"id"`
	WithdrawalNo  string  `json:"withdrawalNo"`
	ApplicantType string  `json:"applicantType"`
	ApplicantId   string  `json:"applicantId"`
	Amount        float64 `json:"amount"`
	BankCard      string  `json:"bankCard"`
	Status        string  `json:"status"`
	AuditTime     string  `json:"auditTime"`
	ProcessTime   string  `json:"processTime"`
	CreateTime    string  `json:"createTime"`
}

type WithdrawalListReq struct {
	ApplicantType string `form:"applicantType,optional"`
	Status        string `form:"status,optional"`
	Page          int    `form:"page,default=1"`
	Size          int    `form:"size,default=20"`
}

type WithdrawalListResp struct {
	Total int64        `json:"total"`
	List  []Withdrawal `json:"list"`
	Page  int          `json:"page"`
	Size  int          `json:"size"`
}

type WithdrawalAuditReq struct {
	Id     int64  `path:"id"`
	Action string `json:"action"`
	Remark string `json:"remark,optional"`
}

type WithdrawalAuditResp struct {
	Id     int64  `json:"id"`
	Status string `json:"status"`
}

type WithdrawalProcessReq struct {
	Id int64 `path:"id"`
}

type WithdrawalProcessResp struct {
	Id     int64  `json:"id"`
	Status string `json:"status"`
}

// CommissionConfig 抽成配置
type CommissionConfig struct {
	Id          int64   `json:"id"`
	BizType     string  `json:"bizType"`
	Rate        float64 `json:"rate"`
	Description string  `json:"description"`
	UpdateTime  string  `json:"updateTime"`
}

type CommissionConfigListReq struct {
	BizType string `form:"bizType,optional"`
}

type CommissionConfigListResp struct {
	List []CommissionConfig `json:"list"`
}

type CommissionConfigUpdateReq struct {
	Id          int64   `path:"id"`
	Rate        float64 `json:"rate"`
	Description string  `json:"description,optional"`
}

type CommissionConfigUpdateResp struct {
	Id int64 `json:"id"`
}

// Overview & Report
type OverviewReq struct {
	StartTime string `form:"startTime,optional"`
	EndTime   string `form:"endTime,optional"`
}

type OverviewResp struct {
	TotalIncome      float64 `json:"totalIncome"`
	TempleIncome     float64 `json:"templeIncome"`
	MasterIncome     float64 `json:"masterIncome"`
	ShopIncome       float64 `json:"shopIncome"`
	CommissionIncome float64 `json:"commissionIncome"`
	PendingWithdraw  int     `json:"pendingWithdraw"`
}

type ReportReq struct {
	StartTime string `form:"startTime"`
	EndTime   string `form:"endTime"`
	Type      string `form:"type,optional"`
	Page      int    `form:"page,default=1"`
	Size      int    `form:"size,default=20"`
}

type ReportResp struct {
	TotalIncome     float64 `json:"totalIncome"`
	TotalSettlement float64 `json:"totalSettlement"`
	TotalWithdrawal float64 `json:"totalWithdrawal"`
	OrderCount      int     `json:"orderCount"`
}

// ============ 法师提现申请 ============

// WithdrawalApplyReq 法师提现申请请求
type WithdrawalApplyReq struct {
	Amount   float64 `json:"amount"`
	BankCard string  `json:"bankCard"`
}

// WithdrawalApplyResp 法师提现申请响应
type WithdrawalApplyResp struct {
	Id            int64   `json:"id"`
	WithdrawalNo  string  `json:"withdrawalNo"`
	ApplicantType string  `json:"applicantType"`
	ApplicantId   string  `json:"applicantId"`
	Amount        float64 `json:"amount"`
	Status        string  `json:"status"`
	CreateTime    string  `json:"createTime"`
}

type ShopReportReq struct {
	StartTime string `form:"startTime,optional"`
	EndTime   string `form:"endTime,optional"`
}

type ShopReportResp struct {
	TotalSales    float64           `json:"totalSales"`
	TotalOrders   int               `json:"totalOrders"`
	AvgOrderValue float64           `json:"avgOrderValue"`
	RefundRate    float64           `json:"refundRate"`
	SalesTrend    []SalesTrendPoint `json:"salesTrend"`
	TopProducts   []TopProduct      `json:"topProducts"`
}

type SalesTrendPoint struct {
	Date   string  `json:"date"`
	Sales  float64 `json:"sales"`
	Orders int     `json:"orders"`
}

type TopProduct struct {
	ProductId   int64   `json:"productId"`
	ProductName string  `json:"productName"`
	Sales       float64 `json:"sales"`
	OrderCount  int     `json:"orderCount"`
}

type PayoutSandboxListReq struct {
	Status string `form:"status,optional"`
	Page   int    `form:"page,default=1"`
	Size   int    `form:"size,default=20"`
}
type PayoutSandboxCreateReq struct {
	RequestID   string `json:"requestId"`
	AmountCents int64  `json:"amountCents"`
}
type PayoutSandboxActionReq struct {
	RequestID string `json:"requestId"`
	Version   int64  `json:"version"`
	Action    string `json:"action"`
	Outcome   string `json:"outcome,optional"`
}
type PayoutSandboxRecord struct {
	ID          string `json:"id"`
	Simulated   bool   `json:"simulated"`
	Recipient   string `json:"recipient"`
	AmountCents int64  `json:"amountCents"`
	Status      string `json:"status"`
	Version     int64  `json:"version"`
	Attempt     int    `json:"attempt"`
	CreatedBy   string `json:"createdBy"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}
type PayoutSandboxEvent struct {
	ID         int64  `json:"id"`
	RequestID  string `json:"requestId"`
	Stage      string `json:"stage"`
	Action     string `json:"action"`
	FromStatus string `json:"fromStatus"`
	ToStatus   string `json:"toStatus"`
	Outcome    string `json:"outcome"`
	Version    int64  `json:"version"`
	Attempt    int    `json:"attempt"`
	ActorID    string `json:"actorId"`
	CreatedAt  string `json:"createdAt"`
}
type PayoutSandboxListResp struct {
	Simulated bool                  `json:"simulated"`
	Total     int64                 `json:"total"`
	Page      int                   `json:"page"`
	Size      int                   `json:"size"`
	List      []PayoutSandboxRecord `json:"list"`
}
type PayoutSandboxDetailResp struct {
	Simulated bool                 `json:"simulated"`
	Record    PayoutSandboxRecord  `json:"record"`
	Events    []PayoutSandboxEvent `json:"events"`
}

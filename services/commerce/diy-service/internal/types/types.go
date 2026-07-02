package types

// DiyDesign DIY设计
type DiyDesign struct {
	Id               int64   `json:"id"`
	DesignNo         string  `json:"designNo"`
	UserId           string  `json:"userId"`
	Name             string  `json:"name"`
	DesignData       string  `json:"designData"`
	TotalPrice       float64 `json:"totalPrice"`
	Status           string  `json:"status"`
	BlessServiceCode string  `json:"blessServiceCode"`
	CreateTime       string  `json:"createTime"`
}

// DiyOrder DIY订单
type DiyOrder struct {
	Id           int64          `json:"id"`
	OrderNo      string         `json:"orderNo"`
	UserId       string         `json:"userId"`
	DesignId     int64          `json:"designId"`
	MaterialFee  float64        `json:"materialFee"`
	BlessFee     float64        `json:"blessFee"`
	TotalFee     float64        `json:"totalFee"`
	Status       string         `json:"status"`
	AddressId    int64          `json:"addressId"`
	Items        []DiyOrderItem `json:"items"`
	BlessingTask BlessingTask   `json:"blessingTask"`
	CreateTime   string         `json:"createTime"`
}

// DiyOrderItem DIY订单明细
type DiyOrderItem struct {
	Id           int64   `json:"id,optional"`
	OrderId      int64   `json:"orderId,optional"`
	MaterialId   int64   `json:"materialId"`
	MaterialName string  `json:"materialName"`
	Spec         string  `json:"spec"`
	UnitPrice    float64 `json:"unitPrice"`
	Quantity     int     `json:"quantity"`
	Subtype      string  `json:"subtype"`
}

// Material 材料库
type Material struct {
	Id           int64   `json:"id"`
	Name         string  `json:"name"`
	Spec         string  `json:"spec"`
	UnitPrice    float64 `json:"unitPrice"`
	Unit         string  `json:"unit"`
	Category     string  `json:"category"`
	FiveElements string  `json:"fiveElements"`
	Image        string  `json:"image"`
	Stock        int     `json:"stock"`
	Status       string  `json:"status"`
}

// BlessingTask 加持任务（字段与 temple/master-service 对齐）
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
}

// BlessingService 加持服务（extra_service 表）
type BlessingService struct {
	Id          int64   `json:"id"`
	ServiceCode string  `json:"serviceCode"`
	ServiceName string  `json:"serviceName"`
	TempleCode  string  `json:"templeCode"`
	TempleName  string  `json:"templeName"`
	MasterCode  string  `json:"masterCode"`
	MasterName  string  `json:"masterName"`
	Price       float64 `json:"price"`
	Description string  `json:"description"`
	Status      string  `json:"status"`
}

// ===== C端请求/响应 =====

type DesignListReq struct {
	Page int `form:"page,default=1"`
	Size int `form:"size,default=20"`
}

type DesignListResp struct {
	Total int64       `json:"total"`
	List  []DiyDesign `json:"list"`
	Page  int         `json:"page"`
	Size  int         `json:"size"`
}

type DesignSaveReq struct {
	UserId           string  `json:"userId"`
	Name             string  `json:"name"`
	DesignData       string  `json:"designData"`
	TotalPrice       float64 `json:"totalPrice"`
	Status           string  `json:"status"`
	BlessServiceCode string  `json:"blessServiceCode"`
}

type DesignSaveResp struct {
	Id int64 `json:"id"`
}

type DesignDetailReq struct {
	Id int64 `path:"id"`
}

type MaterialListReq struct {
	Category string `form:"category,optional"`
	Page     int    `form:"page,default=1"`
	Size     int    `form:"size,default=50"`
}

type MaterialListResp struct {
	Total int64      `json:"total"`
	List  []Material `json:"list"`
	Page  int        `json:"page"`
	Size  int        `json:"size"`
}

type DiyOrderCreateReq struct {
	UserId           string         `json:"userId"`
	DesignId         int64          `json:"designId"`
	Items            []DiyOrderItem `json:"items"`
	BlessServiceCode string         `json:"blessServiceCode"`
	AddressId        int64          `json:"addressId"`
}

type DiyOrderCreateResp struct {
	Id      int64  `json:"id"`
	OrderNo string `json:"orderNo"`
}

type DiyOrderListReq struct {
	UserId string `form:"userId"`
	Status string `form:"status,optional"`
	Page   int    `form:"page,default=1"`
	Size   int    `form:"size,default=20"`
}

type DiyOrderListResp struct {
	Total int64      `json:"total"`
	List  []DiyOrder `json:"list"`
	Page  int        `json:"page"`
	Size  int        `json:"size"`
}

type DiyOrderDetailReq struct {
	Id int64 `path:"id"`
}

// ===== 商城台请求/响应 =====

type AdminDiyOrderListReq struct {
	Status string `form:"status,optional"`
	Page   int    `form:"page,default=1"`
	Size   int    `form:"size,default=20"`
}

type AdminDiyOrderListResp struct {
	Total int64      `json:"total"`
	List  []DiyOrder `json:"list"`
	Page  int        `json:"page"`
	Size  int        `json:"size"`
}

type AdminDiyOrderDetailReq struct {
	Id int64 `path:"id"`
}

type AdminDiyOrderReviewReq struct {
	Id     int64  `path:"id"`
	Action string `json:"action"`
	Reason string `json:"reason"`
}

type AdminDiyOrderMakeCompleteReq struct {
	Id int64 `path:"id"`
}

type AdminDiyOrderShipReq struct {
	Id             int64  `path:"id"`
	ExpressCompany string `json:"expressCompany"`
	TrackingNo     string `json:"trackingNo"`
}

type AdminMaterialListReq struct {
	Category string `form:"category,optional"`
	Keyword  string `form:"keyword,optional"`
	Page     int    `form:"page,default=1"`
	Size     int    `form:"size,default=20"`
}

type AdminMaterialListResp struct {
	Total int64      `json:"total"`
	List  []Material `json:"list"`
	Page  int        `json:"page"`
	Size  int        `json:"size"`
}

type AdminMaterialCreateReq struct {
	Name         string  `json:"name"`
	Spec         string  `json:"spec"`
	UnitPrice    float64 `json:"unitPrice"`
	Unit         string  `json:"unit"`
	Category     string  `json:"category"`
	FiveElements string  `json:"fiveElements"`
	Image        string  `json:"image"`
	Stock        int     `json:"stock"`
}

type AdminMaterialCreateResp struct {
	Id int64 `json:"id"`
}

type AdminMaterialUpdateReq struct {
	Id           int64   `path:"id"`
	Name         string  `json:"name"`
	Spec         string  `json:"spec"`
	UnitPrice    float64 `json:"unitPrice"`
	Unit         string  `json:"unit"`
	Category     string  `json:"category"`
	FiveElements string  `json:"fiveElements"`
	Image        string  `json:"image"`
	Stock        int     `json:"stock"`
}

type AdminMaterialStatusReq struct {
	Id     int64  `path:"id"`
	Status string `json:"status"`
}

type AdminBlessingServiceListReq struct {
	Page int `form:"page,default=1"`
	Size int `form:"size,default=20"`
}

type AdminBlessingServiceListResp struct {
	Total int64             `json:"total"`
	List  []BlessingService `json:"list"`
	Page  int               `json:"page"`
	Size  int               `json:"size"`
}

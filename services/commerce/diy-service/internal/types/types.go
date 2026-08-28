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
	Id                  int64          `json:"id"`
	OrderNo             string         `json:"orderNo"`
	UserId              string         `json:"userId"`
	DesignId            int64          `json:"designId"`
	MaterialFee         float64        `json:"materialFee"`
	BlessFee            float64        `json:"blessFee"`
	TotalFee            float64        `json:"totalFee"`
	Status              string         `json:"status"`
	PaymentStatus       string         `json:"paymentStatus"`
	AddressId           int64          `json:"addressId"`
	Source              string         `json:"source"`
	CreatorId           string         `json:"creatorId"`
	CreatorShareRate    float64        `json:"creatorShareRate"`
	OriginalMaterialFee float64        `json:"originalMaterialFee"`
	PriceChanged        bool           `json:"priceChanged"`
	DesignSnapshot      string         `json:"designSnapshot"`
	PricingSnapshot     string         `json:"pricingSnapshot"`
	Items               []DiyOrderItem `json:"items"`
	BlessingTask        BlessingTask   `json:"blessingTask"`
	CreateTime          string         `json:"createTime"`
}

// DiyOrderItem DIY订单明细
type DiyOrderItem struct {
	Id           int64   `json:"id,optional"`
	OrderId      int64   `json:"orderId,optional"`
	MaterialId   int64   `json:"materialId"`
	SkuId        int64   `json:"skuId,optional"`
	MaterialName string  `json:"materialName"`
	Spec         string  `json:"spec,optional"`
	UnitPrice    float64 `json:"unitPrice"`
	Quantity     int     `json:"quantity"`
	Subtype      string  `json:"subtype,optional"`
}

// MaterialDetailReq 材料详情
type MaterialDetailReq struct {
	Id int64 `path:"id"`
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
	MaterialType string  `json:"materialType"`
	Shape        string  `json:"shape"`
	DiameterMm   float64 `json:"diameterMm"`
	ColorHex     string  `json:"colorHex"`
	TextureKey   string  `json:"textureKey"`
	Finish       string  `json:"finish"`
	Translucency float64 `json:"translucency"`
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

// MyDesignItem 我的设计（含最新订单信息）
type MyDesignItem struct {
	Id               int64   `json:"id"`
	DesignNo         string  `json:"designNo"`
	Name             string  `json:"name"`
	DesignData       string  `json:"designData"`
	TotalPrice       float64 `json:"totalPrice"`
	Status           string  `json:"status"`
	BlessServiceCode string  `json:"blessServiceCode,omitempty"`
	CreateTime       string  `json:"createTime"`
	UpdateTime       string  `json:"updateTime"`
	OrderNo          string  `json:"orderNo,omitempty"`
	OrderStatus      string  `json:"orderStatus,omitempty"`
}

// MyDesignListResp 我的设计列表
type MyDesignListResp struct {
	Total int64          `json:"total"`
	List  []MyDesignItem `json:"list"`
	Page  int            `json:"page"`
	Size  int            `json:"size"`
}

type DesignListResp struct {
	Total int64       `json:"total"`
	List  []DiyDesign `json:"list"`
	Page  int         `json:"page"`
	Size  int         `json:"size"`
}

type BlessingServiceListReq struct {
	Page int `form:"page,default=1"`
	Size int `form:"size,default=20"`
}

type BlessingServiceListResp struct {
	Total int64             `json:"total"`
	List  []BlessingService `json:"list"`
	Page  int               `json:"page"`
	Size  int               `json:"size"`
}

type DesignSaveReq struct {
	UserId           string  `json:"userId"`
	Name             string  `json:"name"`
	DesignData       string  `json:"designData"`
	TotalPrice       float64 `json:"totalPrice"`
	Status           string  `json:"status"`
	BlessServiceCode string  `json:"blessServiceCode,optional"`
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
	BlessServiceCode string         `json:"blessServiceCode,optional"`
	AddressId        int64          `json:"addressId"`
}

type DiyDesignOrderCreateReq struct {
	Id               int64  `path:"id"`
	UserId           string `json:"userId"`
	BlessServiceCode string `json:"blessServiceCode,optional"`
	AddressId        int64  `json:"addressId"`
}

type DiyOrderCreateResp struct {
	Id                  int64          `json:"id"`
	OrderNo             string         `json:"orderNo"`
	UserId              string         `json:"userId"`
	DesignId            int64          `json:"designId"`
	MaterialFee         float64        `json:"materialFee"`
	BlessFee            float64        `json:"blessFee"`
	TotalFee            float64        `json:"totalFee"`
	Status              string         `json:"status"`
	PaymentStatus       string         `json:"paymentStatus"`
	AddressId           int64          `json:"addressId"`
	Items               []DiyOrderItem `json:"items"`
	Source              string         `json:"source"`
	CreatorId           string         `json:"creatorId"`
	CreatorShareRate    float64        `json:"creatorShareRate"`
	OriginalMaterialFee float64        `json:"originalMaterialFee"`
	PriceChanged        bool           `json:"priceChanged"`
	DesignSnapshot      string         `json:"designSnapshot"`
	PricingSnapshot     string         `json:"pricingSnapshot"`
	CreateTime          string         `json:"createTime"`
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
	Reason string `json:"reason,optional"`
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
	MaterialType string  `json:"materialType"`
	Shape        string  `json:"shape"`
	DiameterMm   float64 `json:"diameterMm"`
	ColorHex     string  `json:"colorHex"`
	TextureKey   string  `json:"textureKey"`
	Finish       string  `json:"finish"`
	Translucency float64 `json:"translucency"`
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
	MaterialType string  `json:"materialType"`
	Shape        string  `json:"shape"`
	DiameterMm   float64 `json:"diameterMm"`
	ColorHex     string  `json:"colorHex"`
	TextureKey   string  `json:"textureKey"`
	Finish       string  `json:"finish"`
	Translucency float64 `json:"translucency"`
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

type AdminBlessingServiceCreateReq struct {
	ServiceName string  `json:"serviceName"`
	TempleCode  string  `json:"templeCode"`
	MasterCode  string  `json:"masterCode"`
	Price       float64 `json:"price"`
	Description string  `json:"description"`
	Status      string  `json:"status"`
}

type AdminBlessingServiceCreateResp struct {
	Id int64 `json:"id"`
}

type AdminBlessingServiceDetailReq struct {
	Id int64 `path:"id"`
}

type AdminBlessingServiceUpdateReq struct {
	Id          int64   `path:"id"`
	ServiceName string  `json:"serviceName"`
	TempleCode  string  `json:"templeCode"`
	MasterCode  string  `json:"masterCode"`
	Price       float64 `json:"price"`
	Description string  `json:"description"`
	Status      string  `json:"status"`
}

type AdminBlessingServiceDeleteReq struct {
	Id int64 `path:"id"`
}

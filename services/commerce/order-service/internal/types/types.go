package types

// ShopOrder 商城订单
type ShopOrder struct {
	Id          int64              `json:"id"`
	OrderNo     string             `json:"orderNo"`
	UserId      string             `json:"userId"`
	TotalAmount float64            `json:"totalAmount"`
	PayAmount   float64            `json:"payAmount"`
	Status      string             `json:"status"`
	AddressId   int64              `json:"addressId"`
	Note        string             `json:"note"`
	Items       []ShopOrderItem    `json:"items"`
	Logistics   ShopOrderLogistics `json:"logistics"`
	CreateTime  string             `json:"createTime"`
}

// ShopOrderItem 订单明细
type ShopOrderItem struct {
	Id          int64   `json:"id,optional"`
	OrderId     int64   `json:"orderId,optional"`
	ProductId   int64   `json:"productId"`
	SkuId       int64   `json:"skuId,optional"`
	ProductName string  `json:"productName"`
	SkuSpec     string  `json:"skuSpec,optional"`
	Price       float64 `json:"price"`
	Quantity    int     `json:"quantity"`
	Image       string  `json:"image,optional"`
}

// ShopOrderLogistics 订单物流
type ShopOrderLogistics struct {
	Id             int64  `json:"id"`
	OrderId        int64  `json:"orderId"`
	ExpressCompany string `json:"expressCompany"`
	TrackingNo     string `json:"trackingNo"`
	ShipTime       string `json:"shipTime"`
}

// ReturnOrder 退换货订单
type ReturnOrder struct {
	Id           int64   `json:"id"`
	ReturnNo     string  `json:"returnNo"`
	OrderId      int64   `json:"orderId"`
	Type         string  `json:"type"`
	Reason       string  `json:"reason"`
	Status       string  `json:"status"`
	RefundAmount float64 `json:"refundAmount"`
	CreateTime   string  `json:"createTime"`
}

// ===== C端请求/响应 =====

type OrderCreateReq struct {
	RequestId string          `json:"requestId"`
	UserId    string          `json:"userId"`
	AddressId int64           `json:"addressId"`
	Note      string          `json:"note"`
	Items     []ShopOrderItem `json:"items"`
}

type OrderCreateResp struct {
	Id          int64   `json:"id"`
	OrderNo     string  `json:"orderNo"`
	TotalAmount float64 `json:"totalAmount"`
	PayAmount   float64 `json:"payAmount"`
}

type OrderListReq struct {
	UserId string `form:"userId,optional"`
	Status string `form:"status,optional"`
	Page   int    `form:"page,default=1"`
	Size   int    `form:"size,default=20"`
}

type OrderListResp struct {
	Total int64       `json:"total"`
	List  []ShopOrder `json:"list"`
	Page  int         `json:"page"`
	Size  int         `json:"size"`
}

type OrderDetailReq struct {
	Id int64 `path:"id"`
}

type OrderConfirmReq struct {
	Id int64 `path:"id"`
}

type OrderReturnReq struct {
	Id     int64  `path:"id"`
	Type   string `json:"type"`
	Reason string `json:"reason"`
}

type OrderReturnResp struct {
	Id       int64  `json:"id"`
	ReturnNo string `json:"returnNo"`
}

// ===== 商城台请求/响应 =====

type AdminOrderListReq struct {
	Status string `form:"status,optional"`
	Page   int    `form:"page,default=1"`
	Size   int    `form:"size,default=20"`
}

type AdminOrderListResp struct {
	Total int64       `json:"total"`
	List  []ShopOrder `json:"list"`
	Page  int         `json:"page"`
	Size  int         `json:"size"`
}

type AdminOrderDetailReq struct {
	Id int64 `path:"id"`
}

type AdminOrderShipReq struct {
	Id             int64  `path:"id"`
	ExpressCompany string `json:"expressCompany"`
	TrackingNo     string `json:"trackingNo"`
}

type AdminReturnListReq struct {
	Status string `form:"status,optional"`
	Page   int    `form:"page,default=1"`
	Size   int    `form:"size,default=20"`
}

type AdminReturnListResp struct {
	Total int64         `json:"total"`
	List  []ReturnOrder `json:"list"`
	Page  int           `json:"page"`
	Size  int           `json:"size"`
}

type AdminReturnReviewReq struct {
	Id     int64  `path:"id"`
	Action string `json:"action"`
	Reason string `json:"reason"`
}

type AdminReturnRefundReq struct {
	Id     int64   `path:"id"`
	Amount float64 `json:"amount"`
}

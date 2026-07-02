package types

// Payment 支付单
type Payment struct {
	Id         int64   `json:"id"`
	PaymentNo  string  `json:"paymentNo"`
	OrderType  string  `json:"orderType"`
	OrderNo    string  `json:"orderNo"`
	Amount     float64 `json:"amount"`
	Channel    string  `json:"channel"`
	Status     string  `json:"status"`
	TradeNo    string  `json:"tradeNo"`
	CreateTime string  `json:"createTime"`
}

// Refund 退款单
type Refund struct {
	Id         int64   `json:"id"`
	RefundNo   string  `json:"refundNo"`
	PaymentId  int64   `json:"paymentId"`
	Amount     float64 `json:"amount"`
	Reason     string  `json:"reason"`
	Status     string  `json:"status"`
	CreateTime string  `json:"createTime"`
}

// ===== C端请求/响应 =====

type PaymentCreateReq struct {
	OrderType string  `json:"orderType"`
	OrderNo   string  `json:"orderNo"`
	Amount    float64 `json:"amount"`
	Channel   string  `json:"channel"`
	UserId    string  `json:"userId"`
}

type PaymentCreateResp struct {
	Id        int64  `json:"id"`
	PaymentNo string `json:"paymentNo"`
	PayUrl    string `json:"payUrl"`
}

type PaymentQueryReq struct {
	Id int64 `path:"id"`
}

// ===== 回调请求/响应 =====

type CallbackWechatReq struct {
	RawBody string `json:"rawBody"`
}

type CallbackAlipayReq struct {
	RawBody string `json:"rawBody"`
}

type CallbackResp struct {
	Code string `json:"code"`
	Msg  string `json:"msg"`
}

// ===== 内部请求/响应 =====

type RefundReq struct {
	PaymentNo string  `json:"paymentNo"`
	Amount    float64 `json:"amount"`
	Reason    string  `json:"reason"`
}

type RefundResp struct {
	Id       int64  `json:"id"`
	RefundNo string `json:"refundNo"`
	Status   string `json:"status"`
}

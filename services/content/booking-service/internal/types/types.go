package types

// ============ 预约基础 ============

// Booking 预约
type Booking struct {
	Id                string  `json:"id"`
	RequestId         string  `json:"requestId"`
	UserId            string  `json:"userId"`
	TempleId          string  `json:"templeId"`
	TempleName        string  `json:"templeName"`
	MasterId          string  `json:"masterId"`
	MasterName        string  `json:"masterName"`
	ServiceId         string  `json:"serviceId"`
	ServiceName       string  `json:"serviceName"`
	BookingDate       string  `json:"bookingDate"`
	SlotCode          string  `json:"slotCode"`
	TimeSlot          string  `json:"timeSlot"`
	ServiceFee        float64 `json:"serviceFee"`
	MeritMoney        float64 `json:"meritMoney"`
	MeritMoneyTier    string  `json:"meritMoneyTier"`
	TotalFee          float64 `json:"totalFee"`
	PriceSnapshot     string  `json:"priceSnapshot"`
	PaymentNo         string  `json:"paymentNo"`
	PaymentChannel    string  `json:"paymentChannel"`
	PaymentStatus     string  `json:"paymentStatus"`
	PaymentExpireTime string  `json:"paymentExpireTime"`
	SlotReserved      int     `json:"-"`
	Status            string  `json:"status"`
	Note              string  `json:"note"`
	CreatedAt         string  `json:"createdAt"`
}

// CreateReq 创建预约请求
type CreateReq struct {
	RequestId      string  `json:"requestId,optional"`
	UserId         string  `json:"userId,optional"`
	TempleId       string  `json:"templeId"`
	TempleName     string  `json:"templeName,optional"`
	MasterId       string  `json:"masterId"`
	MasterName     string  `json:"masterName,optional"`
	ServiceId      string  `json:"serviceId"`
	ServiceName    string  `json:"serviceName,optional"`
	BookingDate    string  `json:"bookingDate"`
	SlotCode       string  `json:"slotCode,optional"`
	TimeSlot       string  `json:"timeSlot,optional"`
	MeritMoney     float64 `json:"meritMoney"`
	MeritMoneyTier string  `json:"meritMoneyTier"`
	Note           string  `json:"note,optional"`
}

// CreateResp 创建预约响应
type CreateResp struct {
	Id            string  `json:"id"`
	Status        string  `json:"status"`
	PaymentStatus string  `json:"paymentStatus"`
	PaymentNo     string  `json:"paymentNo"`
	ServiceFee    float64 `json:"serviceFee"`
	MeritMoney    float64 `json:"meritMoney"`
	TotalFee      float64 `json:"totalFee"`
	Simulated     bool    `json:"simulated"`
}

type AvailabilityReq struct {
	TempleId    string `form:"templeId"`
	ServiceId   string `form:"serviceId"`
	BookingDate string `form:"date"`
}
type AvailableSlot struct {
	SlotCode  string `json:"slotCode"`
	Label     string `json:"label"`
	TimeRange string `json:"timeRange"`
	Capacity  int    `json:"capacity"`
	Remaining int    `json:"remaining"`
	Available bool   `json:"available"`
}
type AvailabilityResp struct {
	TempleId    string          `json:"templeId"`
	ServiceId   string          `json:"serviceId"`
	ServiceName string          `json:"serviceName"`
	BookingDate string          `json:"bookingDate"`
	ServiceFee  float64         `json:"serviceFee"`
	Slots       []AvailableSlot `json:"slots"`
}
type PayReq struct {
	Id string `path:"id"`
}

// ============ 付费预约聊天 ============

type ChatListReq struct {
	Page int `form:"page,default=1"`
	Size int `form:"size,default=20"`
}

type ChatConversation struct {
	BookingId     string `json:"bookingId"`
	PeerId        string `json:"peerId"`
	PeerName      string `json:"peerName"`
	PeerAvatar    string `json:"peerAvatar"`
	TempleName    string `json:"templeName"`
	ServiceName   string `json:"serviceName"`
	BookingDate   string `json:"bookingDate"`
	LastMessage   string `json:"lastMessage"`
	LastMessageAt string `json:"lastMessageAt"`
	CanChat       bool   `json:"canChat"`
}

type ChatListResp struct {
	Total int64              `json:"total"`
	List  []ChatConversation `json:"list"`
	Page  int                `json:"page"`
	Size  int                `json:"size"`
}

type ChatMessageListReq struct {
	Id   string `path:"id"`
	Page int    `form:"page,default=1"`
	Size int    `form:"size,default=50"`
}

type ChatMessage struct {
	Id              int64  `json:"id"`
	BookingId       string `json:"bookingId"`
	ClientMessageId string `json:"clientMessageId"`
	SenderType      string `json:"senderType"`
	SenderId        string `json:"senderId"`
	ReceiverId      string `json:"receiverId"`
	Content         string `json:"content"`
	Status          string `json:"status"`
	CreateTime      string `json:"createTime"`
}

type ChatMessageListResp struct {
	Total int64         `json:"total"`
	List  []ChatMessage `json:"list"`
	Page  int           `json:"page"`
	Size  int           `json:"size"`
}

type ChatMessageSendReq struct {
	Id              string `path:"id"`
	ClientMessageId string `json:"clientMessageId"`
	Content         string `json:"content"`
}

// ListReq 列表请求
type ListReq struct {
	UserId   string `form:"userId,optional"`
	Status   string `form:"status,optional"`
	TempleId string `form:"templeId,optional"`
	Page     int    `form:"page,default=1"`
	Size     int    `form:"size,default=20"`
}

// ListResp 列表响应
type ListResp struct {
	Total int64     `json:"total"`
	List  []Booking `json:"list"`
	Page  int       `json:"page"`
	Size  int       `json:"size"`
}

// DetailReq 详情请求
type DetailReq struct {
	Id string `path:"id"`
}

// StatusReq 状态流转请求
type StatusReq struct {
	Id     string `path:"id"`
	Status string `json:"status"`
}

// StatusResp 状态流转响应
type StatusResp struct {
	Id     string `json:"id"`
	Status string `json:"status"`
}

// ============ 评价 ============

// BookingReview 预约评价
type BookingReview struct {
	Id          int64    `json:"id"`
	BookingId   string   `json:"bookingId"`
	UserId      string   `json:"userId"`
	Rating      int      `json:"rating"`
	Content     string   `json:"content"`
	Images      []string `json:"images"`
	MasterReply string   `json:"masterReply"`
	CreateTime  string   `json:"createTime"`
}

// ReviewCreateReq 创建评价请求
type ReviewCreateReq struct {
	Id      string   `path:"id"`
	Rating  int      `json:"rating"`
	Content string   `json:"content"`
	Images  []string `json:"images,optional"`
}

// ReviewCreateResp 创建评价响应
type ReviewCreateResp struct {
	ReviewId int64 `json:"reviewId"`
}

// ReviewDetailReq 评价详情请求
type ReviewDetailReq struct {
	Id string `path:"id"`
}

// ReviewReplyReq 法师回复评价请求
type ReviewReplyReq struct {
	Id          string `path:"id"`
	MasterReply string `json:"masterReply"`
}

// ============ 管理台 ============

// AdminBookingListReq 管理台预约列表请求
type AdminBookingListReq struct {
	TempleId string `form:"templeId"`
	Status   string `form:"status,optional"`
	MasterId string `form:"masterId,optional"`
	Page     int    `form:"page,default=1"`
	Size     int    `form:"size,default=20"`
}

// AdminBookingListResp 管理台预约列表响应
type AdminBookingListResp struct {
	Total int64     `json:"total"`
	List  []Booking `json:"list"`
	Page  int       `json:"page"`
	Size  int       `json:"size"`
}

// AdminBookingActionReq 管理台预约操作请求
type AdminBookingActionReq struct {
	Id     string `path:"id"`
	Remark string `json:"remark,optional"`
}

// StatusLogReq 状态日志请求
type StatusLogReq struct {
	Id string `path:"id"`
}

// BookingStatusLog 状态变更日志
type BookingStatusLog struct {
	Id           int64  `json:"id"`
	BookingId    string `json:"bookingId"`
	FromStatus   string `json:"fromStatus"`
	ToStatus     string `json:"toStatus"`
	OperatorId   string `json:"operatorId"`
	OperatorType string `json:"operatorType"`
	Remark       string `json:"remark"`
	CreateTime   string `json:"createTime"`
}

// StatusLogResp 状态日志响应
type StatusLogResp struct {
	List []BookingStatusLog `json:"list"`
}

// ============ 法师工作台 - 预约 ============

// MasterBookingListReq 法师预约列表请求（masterId 从 JWT 获取，无需前端传递）
type MasterBookingListReq struct {
	Status string `form:"status,optional"`
	Page   int    `form:"page,default=1"`
	Size   int    `form:"size,default=20"`
}

// MasterBookingListResp 法师预约列表响应
type MasterBookingListResp struct {
	Total int64     `json:"total"`
	List  []Booking `json:"list"`
	Page  int       `json:"page"`
	Size  int       `json:"size"`
}

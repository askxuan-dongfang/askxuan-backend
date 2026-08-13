package types

// ===== 通用 =====

// 分页请求基类字段
type PageReq struct {
	Page int `form:"page,default=1"`
	Size int `form:"size,default=20"`
}

// ===== Banner =====

// Banner 首页轮播
type Banner struct {
	Id        int64  `json:"id"`
	Title     string `json:"title"`
	ImageUrl  string `json:"imageUrl"`
	LinkType  string `json:"linkType"` // temple/master/product/diy/ad_landing
	LinkValue string `json:"linkValue"`
	Sort      int    `json:"sort"`
	Status    string `json:"status"` // enabled/disabled
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	CreatedAt string `json:"createdAt"`
}

// BannerListReq Banner 列表请求
type BannerListReq struct {
	Status string `form:"status,optional"`
	Page   int    `form:"page,default=1"`
	Size   int    `form:"size,default=20"`
}

// BannerListResp Banner 列表响应
type BannerListResp struct {
	Total int64    `json:"total"`
	List  []Banner `json:"list"`
	Page  int      `json:"page"`
	Size  int      `json:"size"`
}

// BannerCreateReq 创建 Banner
type BannerCreateReq struct {
	Title     string `json:"title"`
	ImageUrl  string `json:"imageUrl"`
	LinkType  string `json:"linkType"`
	LinkValue string `json:"linkValue"`
	Sort      int    `json:"sort,optional"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
}

// BannerUpdateReq 更新 Banner
type BannerUpdateReq struct {
	Id        int64  `path:"id"`
	Title     string `json:"title,optional"`
	ImageUrl  string `json:"imageUrl,optional"`
	LinkType  string `json:"linkType,optional"`
	LinkValue string `json:"linkValue,optional"`
	Sort      int    `json:"sort,optional"`
	Status    string `json:"status,optional"`
	StartTime string `json:"startTime,optional"`
	EndTime   string `json:"endTime,optional"`
}

// IdResp 通用 ID 响应
type IdResp struct {
	Id int64 `json:"id"`
}

// IdReq 通用 ID 路径请求
type IdReq struct {
	Id int64 `path:"id"`
}

// ===== Recommend 推荐位 =====

// Recommend 推荐位
type Recommend struct {
	Id        int64  `json:"id"`
	Type      string `json:"type"` // temple/master/product
	TargetId  string `json:"targetId"`
	Sort      int    `json:"sort"`
	Status    string `json:"status"` // enabled/disabled
	CreatedAt string `json:"createdAt"`
}

// RecommendListReq 推荐位列表请求
type RecommendListReq struct {
	Type   string `form:"type,optional"`
	Status string `form:"status,optional"`
	Page   int    `form:"page,default=1"`
	Size   int    `form:"size,default=20"`
}

// RecommendListResp 推荐位列表响应
type RecommendListResp struct {
	Total int64       `json:"total"`
	List  []Recommend `json:"list"`
	Page  int         `json:"page"`
	Size  int         `json:"size"`
}

// RecommendUpdateReq 更新推荐位
type RecommendUpdateReq struct {
	Id       int64  `path:"id"`
	Type     string `json:"type,optional"`
	TargetId string `json:"targetId,optional"`
	Sort     int    `json:"sort,optional"`
	Status   string `json:"status,optional"`
}

// ===== Activity 活动 =====

// Activity 营销活动
type Activity struct {
	Id        int64  `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"` // limited_discount/festival/temple_event
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	Config    string `json:"config"` // JSON 字符串
	Status    string `json:"status"` // enabled/disabled
	CreatedAt string `json:"createdAt"`
}

// ActivityListReq 活动列表请求
type ActivityListReq struct {
	Status string `form:"status,optional"`
	Type   string `form:"type,optional"`
	Page   int    `form:"page,default=1"`
	Size   int    `form:"size,default=20"`
}

// ActivityListResp 活动列表响应
type ActivityListResp struct {
	Total int64      `json:"total"`
	List  []Activity `json:"list"`
	Page  int        `json:"page"`
	Size  int        `json:"size"`
}

// ActivityCreateReq 创建活动
type ActivityCreateReq struct {
	Name      string `json:"name"`
	Type      string `json:"type"`
	StartTime string `json:"startTime"`
	EndTime   string `json:"endTime"`
	Config    string `json:"config,optional"`
}

// ActivityUpdateReq 更新活动
type ActivityUpdateReq struct {
	Id        int64  `path:"id"`
	Name      string `json:"name,optional"`
	Type      string `json:"type,optional"`
	StartTime string `json:"startTime,optional"`
	EndTime   string `json:"endTime,optional"`
	Config    string `json:"config,optional"`
	Status    string `json:"status,optional"`
}

// ===== Coupon 优惠券 =====

// Coupon 优惠券
type Coupon struct {
	Id            int64   `json:"id"`
	CouponNo      string  `json:"couponNo"`
	Name          string  `json:"name"`
	Type          string  `json:"type"`      // full_reduce/discount/new_user/category
	Value         float64 `json:"value"`     // 满减金额 / 折扣率
	MinAmount     float64 `json:"minAmount"` // 最低消费
	CategoryId    string  `json:"categoryId"`
	StartTime     string  `json:"startTime"`
	EndTime       string  `json:"endTime"`
	TotalCount    int     `json:"totalCount"`
	ReceivedCount int     `json:"receivedCount"`
	Status        string  `json:"status"` // enabled/disabled
	CreatedAt     string  `json:"createdAt"`
}

// CouponListReq 优惠券列表请求
type CouponListReq struct {
	Status string `form:"status,optional"`
	Type   string `form:"type,optional"`
	Page   int    `form:"page,default=1"`
	Size   int    `form:"size,default=20"`
}

// CouponListResp 优惠券列表响应
type CouponListResp struct {
	Total int64    `json:"total"`
	List  []Coupon `json:"list"`
	Page  int      `json:"page"`
	Size  int      `json:"size"`
}

// CouponCreateReq 创建优惠券
type CouponCreateReq struct {
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	Value      float64 `json:"value"`
	MinAmount  float64 `json:"minAmount,optional"`
	CategoryId string  `json:"categoryId,optional"`
	StartTime  string  `json:"startTime"`
	EndTime    string  `json:"endTime"`
	TotalCount int     `json:"totalCount"`
}

// CouponUpdateReq 更新优惠券
type CouponUpdateReq struct {
	Id         int64   `path:"id"`
	Name       string  `json:"name,optional"`
	Type       string  `json:"type,optional"`
	Value      float64 `json:"value,optional"`
	MinAmount  float64 `json:"minAmount,optional"`
	CategoryId string  `json:"categoryId,optional"`
	StartTime  string  `json:"startTime,optional"`
	EndTime    string  `json:"endTime,optional"`
	TotalCount int     `json:"totalCount,optional"`
	Status     string  `json:"status,optional"`
}

// CouponReceiveReq 领取优惠券
type CouponReceiveReq struct {
	Id     int64  `path:"id"`
	UserId string `form:"userId"`
}

// CouponRecord 优惠券领取记录（我的优惠券）
type CouponRecord struct {
	Id        int64  `json:"id"`
	CouponId  int64  `json:"couponId"`
	CouponNo  string `json:"couponNo"`
	UserId    string `json:"userId"`
	Status    string `json:"status"` // unused/used/expired
	OrderNo   string `json:"orderNo"`
	UseTime   string `json:"useTime"`
	CreatedAt string `json:"createdAt"`
	// 冗余优惠券摘要信息，便于 C 端展示
	Name      string  `json:"name"`
	Type      string  `json:"type"`
	Value     float64 `json:"value"`
	MinAmount float64 `json:"minAmount"`
	EndTime   string  `json:"endTime"`
}

// MyCouponReq 我的优惠券列表请求
type MyCouponReq struct {
	UserId string `form:"userId,optional"`
	Status string `form:"status,optional"` // unused/used/expired
	Page   int    `form:"page,default=1"`
	Size   int    `form:"size,default=20"`
}

// MyCouponResp 我的优惠券列表响应
type MyCouponResp struct {
	Total int64          `json:"total"`
	List  []CouponRecord `json:"list"`
	Page  int            `json:"page"`
	Size  int            `json:"size"`
}

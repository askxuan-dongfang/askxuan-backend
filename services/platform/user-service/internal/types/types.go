package types

// ============ 用户资料 ============

// UserProfile 用户资料
type UserProfile struct {
	UserId   int64  `json:"userId"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Mobile   string `json:"mobile"`
	Gender   string `json:"gender"`
	Birthday string `json:"birthday"`
	Region   string `json:"region"`
	Bio      string `json:"bio"`
}

// RegisterReq 注册请求
type RegisterReq struct {
	Mobile   string `json:"mobile"`
	Code     string `json:"code"`
	Nickname string `json:"nickname,optional"`
}

// RegisterResp 注册响应
type RegisterResp struct {
	UserId int64 `json:"userId"`
}

// ProfileReq 查询资料请求（userId 从 JWT 取）
type ProfileReq struct {
	UserId int64 `json:"userId"`
}

// UpdateProfileReq 更新资料请求
// 注：UserId 加 optional 以兼容 httpx.Parse（handler 从 X-User-Id header 注入，请求体不带此字段）
type UpdateProfileReq struct {
	UserId   int64  `json:"userId,optional"`
	Nickname string `json:"nickname,optional"`
	Avatar   string `json:"avatar,optional"`
	Gender   string `json:"gender,optional"`
	Birthday string `json:"birthday,optional"`
	Region   string `json:"region,optional"`
	Bio      string `json:"bio,optional"`
}

// ============ 收货地址 ============

// UserAddress 收货地址
type UserAddress struct {
	Id         int64  `json:"id"`
	UserId     int64  `json:"userId"`
	Name       string `json:"name"`
	Phone      string `json:"phone"`
	Province   string `json:"province"`
	City       string `json:"city"`
	District   string `json:"district"`
	Detail     string `json:"detail"`
	IsDefault  bool   `json:"isDefault"`
	CreateTime string `json:"createTime"`
}

// AddressListResp 地址列表响应
type AddressListResp struct {
	List []UserAddress `json:"list"`
}

// AddressCreateReq 添加地址请求
type AddressCreateReq struct {
	Name      string `json:"name"`
	Phone     string `json:"phone"`
	Province  string `json:"province"`
	City      string `json:"city"`
	District  string `json:"district"`
	Detail    string `json:"detail"`
	IsDefault bool   `json:"isDefault,optional"`
}

// AddressCreateResp 添加地址响应
type AddressCreateResp struct {
	Id int64 `json:"id"`
}

// AddressUpdateReq 更新地址请求
type AddressUpdateReq struct {
	Id        int64  `path:"id"`
	Name      string `json:"name,optional"`
	Phone     string `json:"phone,optional"`
	Province  string `json:"province,optional"`
	City      string `json:"city,optional"`
	District  string `json:"district,optional"`
	Detail    string `json:"detail,optional"`
	IsDefault bool   `json:"isDefault,optional"`
}

// AddressDeleteReq 删除地址请求
type AddressDeleteReq struct {
	Id int64 `path:"id"`
}

// ============ 平台用户管理 ============

// AdminUserListReq 平台用户列表请求
type AdminUserListReq struct {
	Keyword string `form:"keyword,optional"`
	Status  string `form:"status,optional"`
	Page    int    `form:"page,default=1"`
	Size    int    `form:"size,default=20"`
}

// AdminUserItem 平台用户列表项
type AdminUserItem struct {
	UserId      int64   `json:"userId"`
	Nickname    string  `json:"nickname"`
	Mobile      string  `json:"mobile"`
	Avatar      string  `json:"avatar"`
	Region      string  `json:"region"`
	Status      string  `json:"status"`
	TotalOrders int     `json:"totalOrders"`
	TotalSpent  float64 `json:"totalSpent"`
	CreateTime  string  `json:"createTime"`
}

// AdminUserListResp 平台用户列表响应
type AdminUserListResp struct {
	Total int64           `json:"total"`
	List  []AdminUserItem `json:"list"`
	Page  int             `json:"page"`
	Size  int             `json:"size"`
}

// AdminUserDetailReq 用户详情请求
type AdminUserDetailReq struct {
	Id int64 `path:"id"`
}

// AdminUserDetailResp 用户详情响应（含画像）
type AdminUserDetailResp struct {
	User           UserProfile `json:"user"`
	PreferenceTags []string    `json:"preferenceTags"`
	TotalOrders    int         `json:"totalOrders"`
	TotalSpent     float64     `json:"totalSpent"`
	LastActiveTime string      `json:"lastActiveTime"`
	Status         string      `json:"status"`
}

// AdminUserStatusReq 封禁/解封请求
type AdminUserStatusReq struct {
	Id     int64  `path:"id"`
	Status string `json:"status"`
}

// AdminUserStatusResp 封禁/解封响应
type AdminUserStatusResp struct {
	Id     int64  `json:"id"`
	Status string `json:"status"`
}

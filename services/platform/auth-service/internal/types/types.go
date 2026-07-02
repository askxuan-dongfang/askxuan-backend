package types

// ============ C端/通用认证 ============

// LoginReq 登录请求
type LoginReq struct {
	Phone    string `json:"phone"`
	Code     string `json:"code,optional"`     // 验证码（手机号登录）
	Account  string `json:"account,optional"`  // 账号（密码登录，可传手机号）
	Password string `json:"password,optional"` // 密码（密码登录）
}

// UserInfo 用户简要信息
type UserInfo struct {
	UserId   int64  `json:"userId"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Mobile   string `json:"mobile"`
}

// LoginResp 登录响应
type LoginResp struct {
	AccessToken  string   `json:"accessToken"`
	RefreshToken string   `json:"refreshToken"`
	ExpiresIn    int64    `json:"expiresIn"`
	UserInfo     UserInfo `json:"userInfo"`
	IMToken      string   `json:"imToken,optional"`
}

// RefreshReq 续期请求
type RefreshReq struct {
	RefreshToken string `json:"refreshToken"`
}

// RefreshResp 续期响应
type RefreshResp struct {
	AccessToken string `json:"accessToken"`
	ExpiresIn   int64  `json:"expiresIn"`
}

// LogoutReq 登出请求
type LogoutReq struct {
	AccessToken string `json:"accessToken,optional"`
}

// LogoutResp 登出响应
type LogoutResp struct {
	Success bool `json:"success"`
}

// ============ 管理台登录 ============

// AdminLoginReq 管理台登录请求
type AdminLoginReq struct {
	Account  string `json:"account"`
	Password string `json:"password"`
}

// ============ 管理台账号管理 ============

// AdminAccount 管理台账号
type AdminAccount struct {
	Id            int64  `json:"id"`
	Account       string `json:"account"`
	Name          string `json:"name"`
	RoleId        int64  `json:"roleId"`
	RoleName      string `json:"roleName,optional"`
	TempleId      string `json:"templeId,optional"`
	MasterId      string `json:"masterId,optional"`
	ShopId        int64  `json:"shopId,optional"`
	Status        string `json:"status"`
	LastLoginTime string `json:"lastLoginTime,optional"`
	CreateTime    string `json:"createTime"`
}

// AdminAccountListReq 账号列表请求
type AdminAccountListReq struct {
	Keyword string `form:"keyword,optional"`
	Status  string `form:"status,optional"`
	Page    int    `form:"page,default=1"`
	Size    int    `form:"size,default=20"`
}

// AdminAccountListResp 账号列表响应
type AdminAccountListResp struct {
	Total int64          `json:"total"`
	List  []AdminAccount `json:"list"`
	Page  int            `json:"page"`
	Size  int            `json:"size"`
}

// AdminAccountCreateReq 创建账号请求
type AdminAccountCreateReq struct {
	Account  string `json:"account"`
	Password string `json:"password"`
	Name     string `json:"name"`
	RoleId   int64  `json:"roleId"`
	TempleId string `json:"templeId,optional"`
	MasterId string `json:"masterId,optional"`
	ShopId   int64  `json:"shopId,optional"`
}

// AdminAccountCreateResp 创建账号响应
type AdminAccountCreateResp struct {
	Id int64 `json:"id"`
}

// AdminAccountUpdateReq 更新账号请求
type AdminAccountUpdateReq struct {
	Id       int64  `path:"id"`
	Name     string `json:"name,optional"`
	RoleId   int64  `json:"roleId,optional"`
	TempleId string `json:"templeId,optional"`
	MasterId string `json:"masterId,optional"`
	ShopId   int64  `json:"shopId,optional"`
}

// AdminAccountStatusReq 启停账号请求
type AdminAccountStatusReq struct {
	Id     int64  `path:"id"`
	Status string `json:"status"`
}

// AdminAccountStatusResp 启停账号响应
type AdminAccountStatusResp struct {
	Id     int64  `json:"id"`
	Status string `json:"status"`
}

// ============ 角色管理 ============

// Role 角色
type Role struct {
	Id          int64  `json:"id"`
	Name        string `json:"name"`
	Code        string `json:"code"`
	Description string `json:"description"`
	CreateTime  string `json:"createTime"`
}

// RoleListResp 角色列表响应
type RoleListResp struct {
	List []Role `json:"list"`
}

// RoleCreateReq 创建角色请求
type RoleCreateReq struct {
	Name        string `json:"name"`
	Code        string `json:"code"`
	Description string `json:"description,optional"`
}

// RoleCreateResp 创建角色响应
type RoleCreateResp struct {
	Id int64 `json:"id"`
}

// RoleUpdateReq 更新角色请求
type RoleUpdateReq struct {
	Id          int64  `path:"id"`
	Name        string `json:"name,optional"`
	Description string `json:"description,optional"`
}

// ============ 权限管理 ============

// Permission 权限
type Permission struct {
	Id       int64  `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	Resource string `json:"resource"`
	Action   string `json:"action"`
}

// PermissionListResp 权限列表响应
type PermissionListResp struct {
	List []Permission `json:"list"`
}

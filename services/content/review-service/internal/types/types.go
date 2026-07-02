package types

// Review 评价
type Review struct {
	Id         int64  `json:"id"`
	ReviewNo   string `json:"reviewNo"`
	UserId     string `json:"userId"`
	TargetType string `json:"targetType"`
	TargetId   string `json:"targetId"`
	Rating     int    `json:"rating"`
	Content    string `json:"content"`
	Images     string `json:"images"`
	Status     string `json:"status"`
	CreateTime string `json:"createTime"`
}

// ReviewReply 评价回复
type ReviewReply struct {
	Id          int64  `json:"id"`
	ReviewId    int64  `json:"reviewId"`
	ReplierType string `json:"replierType"`
	ReplierId   string `json:"replierId"`
	Content     string `json:"content"`
	CreateTime  string `json:"createTime"`
}

// ReviewReport 评价举报
type ReviewReport struct {
	Id           int64  `json:"id"`
	ReviewId     int64  `json:"reviewId"`
	ReporterId   string `json:"reporterId"`
	Reason       string `json:"reason"`
	Status       string `json:"status"`
	HandleResult string `json:"handleResult"`
	CreateTime   string `json:"createTime"`
}

// C端 - 提交评价
type CreateReviewReq struct {
	UserId     string `json:"userId"`
	TargetType string `json:"targetType"`
	TargetId   string `json:"targetId"`
	Rating     int    `json:"rating"`
	Content    string `json:"content"`
	Images     string `json:"images,optional"`
}

type CreateReviewResp struct {
	Id     int64  `json:"id"`
	Status string `json:"status"`
}

// C端 - 评价列表
type ReviewListReq struct {
	TargetType string `form:"targetType,optional"`
	TargetId   string `form:"targetId,optional"`
	UserId     string `form:"userId,optional"`
	Rating     int    `form:"rating,optional"`
	Page       int    `form:"page,default=1"`
	Size       int    `form:"size,default=20"`
}

type ReviewListResp struct {
	Total int64    `json:"total"`
	List  []Review `json:"list"`
	Page  int      `json:"page"`
	Size  int      `json:"size"`
}

// 评价详情
type ReviewDetailReq struct {
	Id int64 `path:"id"`
}

// 管理台 - 评价列表
type AdminReviewListReq struct {
	TargetType string `form:"targetType,optional"`
	TargetId   string `form:"targetId,optional"`
	Status     string `form:"status,optional"`
	Rating     int    `form:"rating,optional"`
	Page       int    `form:"page,default=1"`
	Size       int    `form:"size,default=20"`
}

type AdminReviewListResp struct {
	Total int64    `json:"total"`
	List  []Review `json:"list"`
	Page  int      `json:"page"`
	Size  int      `json:"size"`
}

// 回复评价
type ReviewReplyReq struct {
	Id          int64  `path:"id"`
	ReplierType string `json:"replierType"`
	ReplierId   string `json:"replierId"`
	Content     string `json:"content"`
}

type ReviewReplyResp struct {
	Id int64 `json:"id"`
}

// 举报评价
type ReviewReportReq struct {
	Id         int64  `path:"id"`
	ReporterId string `json:"reporterId"`
	Reason     string `json:"reason"`
}

type ReviewReportResp struct {
	Id int64 `json:"id"`
}

// 平台 - 举报列表
type ReportListReq struct {
	Status string `form:"status,optional"`
	Page   int    `form:"page,default=1"`
	Size   int    `form:"size,default=20"`
}

type ReportListResp struct {
	Total int64          `json:"total"`
	List  []ReviewReport `json:"list"`
	Page  int            `json:"page"`
	Size  int            `json:"size"`
}

// 平台 - 处理举报
type ReportHandleReq struct {
	Id           int64  `path:"id"`
	HandleResult string `json:"handleResult"`
	Remark       string `json:"remark,optional"`
}

type ReportHandleResp struct {
	Id     int64  `json:"id"`
	Status string `json:"status"`
}

package types

type Asset struct {
	Id        int64  `json:"id"`
	MediaId   int64  `json:"mediaId"`
	AssetType string `json:"assetType"`
	Sort      int    `json:"sort"`
}

type Post struct {
	Id           string  `json:"id"`
	MasterId     string  `json:"masterId"`
	OwnerId      string  `json:"ownerId,omitempty"`
	Type         string  `json:"type"`
	Title        string  `json:"title"`
	Content      string  `json:"content"`
	CoverMediaId int64   `json:"coverMediaId"`
	BeliefCode   string  `json:"beliefCode"`
	Status       string  `json:"status"`
	AuditRemark  string  `json:"auditRemark,omitempty"`
	LikeCount    int64   `json:"likeCount"`
	CommentCount int64   `json:"commentCount"`
	Liked        bool    `json:"liked"`
	Assets       []Asset `json:"assets"`
	CreateTime   string  `json:"createTime"`
	UpdateTime   string  `json:"updateTime"`
}

type Comment struct {
	Id          string `json:"id"`
	PostId      string `json:"postId"`
	UserId      string `json:"userId"`
	Content     string `json:"content"`
	Status      string `json:"status"`
	AuditRemark string `json:"auditRemark,omitempty"`
	CreateTime  string `json:"createTime"`
}

type FeedReq struct {
	Type       string `form:"type,optional"`
	BeliefCode string `form:"beliefCode,optional"`
	Page       int    `form:"page,optional"`
	Size       int    `form:"size,optional"`
}
type PostDetailReq struct {
	Id string `path:"id"`
}
type PostListResp struct {
	Total int64  `json:"total"`
	List  []Post `json:"list"`
	Page  int    `json:"page"`
	Size  int    `json:"size"`
}
type CommentListReq struct {
	Id   string `path:"id"`
	Page int    `form:"page,optional"`
	Size int    `form:"size,optional"`
}
type CommentListResp struct {
	Total int64     `json:"total"`
	List  []Comment `json:"list"`
	Page  int       `json:"page"`
	Size  int       `json:"size"`
}
type CommentCreateReq struct {
	Id      string `path:"id"`
	UserId  string `json:"userId,optional"`
	Content string `json:"content"`
}
type LikeReq struct {
	Id     string `path:"id"`
	UserId string `json:"userId,optional"`
}
type LikeResp struct {
	Liked     bool  `json:"liked"`
	LikeCount int64 `json:"likeCount"`
}
type FollowReq struct {
	Id     string `path:"id"`
	UserId string `json:"userId,optional"`
}
type FollowResp struct {
	Following bool `json:"following"`
}

type PostWriteReq struct {
	Id           string  `json:"-"`
	OwnerId      string  `json:"ownerId,optional"`
	MasterId     string  `json:"masterId,optional"`
	Type         string  `json:"type"`
	Title        string  `json:"title"`
	Content      string  `json:"content,optional"`
	CoverMediaId int64   `json:"coverMediaId,optional"`
	BeliefCode   string  `json:"beliefCode,optional"`
	Assets       []Asset `json:"assets,optional"`
	Submit       bool    `json:"submit,optional"`
}
type MasterPostListReq struct {
	Status   string `form:"status,optional"`
	OwnerId  string `form:"ownerId,optional"`
	MasterId string `form:"masterId,optional"`
	Page     int    `form:"page,optional"`
	Size     int    `form:"size,optional"`
}
type PostStatusReq struct {
	Id      string `path:"id"`
	OwnerId string `json:"ownerId,optional"`
	Status  string `json:"status"`
}
type AdminListReq struct {
	Status string `form:"status,optional"`
	Page   int    `form:"page,optional"`
	Size   int    `form:"size,optional"`
}
type AdminReviewReq struct {
	Id        string `path:"id"`
	AuditorId string `json:"auditorId,optional"`
	Remark    string `json:"remark,optional"`
}
type AdminCommentListReq struct {
	Status string `form:"status,optional"`
	Page   int    `form:"page,optional"`
	Size   int    `form:"size,optional"`
}

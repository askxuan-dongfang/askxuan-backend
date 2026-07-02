package model

import (
	"sync"
)

// 回复者类型
const (
	ReplierTypeTempleAdmin = "temple_admin"
	ReplierTypeMaster      = "master"
	ReplierTypePlatform    = "platform"
)

// ReviewReply 评价回复结构体
type ReviewReply struct {
	Id          int64  `json:"id"`
	ReviewId    int64  `json:"reviewId"`
	ReplierType string `json:"replierType"`
	ReplierId   string `json:"replierId"`
	Content     string `json:"content"`
	CreateTime  string `json:"createTime"`
}

// ---- 内存存储（MVP 阶段不连 DB）----

type reviewReplyStore struct {
	mu   sync.RWMutex
	list []ReviewReply
	seq  int64
}

var globalReviewReplyStore = &reviewReplyStore{
	list: []ReviewReply{
		{
			Id:          1,
			ReviewId:    1,
			ReplierType: ReplierTypeMaster,
			ReplierId:   "M002",
			Content:     "感谢您的评价，愿福生无量。",
			CreateTime:  "2026-06-20 19:00:00",
		},
	},
	seq: 1,
}

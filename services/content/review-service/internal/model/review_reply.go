package model

import (
	"sync"
	"time"
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

// CreateReply 新建评价回复，seq 自增，设置 createTime，追加到 store
func CreateReply(reply ReviewReply) ReviewReply {
	globalReviewReplyStore.mu.Lock()
	defer globalReviewReplyStore.mu.Unlock()

	globalReviewReplyStore.seq++
	reply.Id = globalReviewReplyStore.seq
	reply.CreateTime = time.Now().Format("2006-01-02 15:04:05")
	globalReviewReplyStore.list = append(globalReviewReplyStore.list, reply)
	return reply
}

// ListRepliesByReviewID 按评价ID查询回复列表
func ListRepliesByReviewID(reviewId int64) []ReviewReply {
	globalReviewReplyStore.mu.RLock()
	defer globalReviewReplyStore.mu.RUnlock()

	result := make([]ReviewReply, 0)
	for _, r := range globalReviewReplyStore.list {
		if r.ReviewId == reviewId {
			result = append(result, r)
		}
	}
	return result
}

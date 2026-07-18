package model

import "context"

const (
	ReplierTypeTempleAdmin = "temple_admin"
	ReplierTypeMaster      = "master"
	ReplierTypePlatform    = "platform"
)

type ReviewReply struct {
	Id          int64  `db:"id" json:"id"`
	ReviewId    int64  `db:"review_id" json:"reviewId"`
	ReplierType string `db:"replier_type" json:"replierType"`
	ReplierId   string `db:"replier_id" json:"replierId"`
	Content     string `db:"content" json:"content"`
	CreateTime  string `db:"create_time" json:"createTime"`
}

func CreateReply(ctx context.Context, reply ReviewReply) (ReviewReply, error) {
	result, err := db.ExecCtx(ctx, "INSERT INTO review_reply(review_id,replier_type,replier_id,content) VALUES(?,?,?,?)", reply.ReviewId, reply.ReplierType, reply.ReplierId, reply.Content)
	if err != nil {
		return ReviewReply{}, err
	}
	reply.Id, err = result.LastInsertId()
	if err != nil {
		return ReviewReply{}, err
	}
	err = db.QueryRowCtx(ctx, &reply, `SELECT id,review_id,replier_type,replier_id,content,DATE_FORMAT(create_time,'%Y-%m-%d %H:%i:%s') create_time FROM review_reply WHERE id=?`, reply.Id)
	return reply, err
}

func ListRepliesByReviewID(ctx context.Context, reviewId int64) ([]ReviewReply, error) {
	var list []ReviewReply
	err := db.QueryRowsCtx(ctx, &list, `SELECT id,review_id,replier_type,replier_id,content,DATE_FORMAT(create_time,'%Y-%m-%d %H:%i:%s') create_time FROM review_reply WHERE review_id=? ORDER BY id`, reviewId)
	return list, err
}

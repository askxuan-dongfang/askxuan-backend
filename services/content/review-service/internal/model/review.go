package model

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

const (
	ReviewStatusNormal = "normal"
	ReviewStatusHidden = "hidden"
)

const (
	TargetTypeBooking   = "booking"
	TargetTypeDiyOrder  = "diy_order"
	TargetTypeShopOrder = "shop_order"
)

type Review struct {
	Id         int64  `db:"id" json:"id"`
	ReviewNo   string `db:"review_no" json:"reviewNo"`
	UserId     string `db:"user_id" json:"userId"`
	TargetType string `db:"target_type" json:"targetType"`
	TargetId   string `db:"target_id" json:"targetId"`
	MasterCode string `db:"master_code" json:"masterCode"`
	Rating     int    `db:"rating" json:"rating"`
	Content    string `db:"content" json:"content"`
	Images     string `db:"images" json:"images"`
	Status     string `db:"status" json:"status"`
	CreateTime string `db:"create_time" json:"createTime"`
}

func ListReviews(ctx context.Context, targetType, targetId, userId string, rating int, status, masterCode string, page, size int) ([]Review, int64, error) {
	where := " WHERE 1=1"
	args := make([]any, 0, 6)
	for _, filter := range []struct{ value, clause string }{
		{targetType, " AND target_type=?"}, {targetId, " AND target_id=?"}, {userId, " AND user_id=?"},
		{status, " AND status=?"}, {masterCode, " AND master_code=?"},
	} {
		if filter.value != "" {
			where += filter.clause
			args = append(args, filter.value)
		}
	}
	if rating > 0 {
		where += " AND rating=?"
		args = append(args, rating)
	}
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	var total int64
	if err := db.QueryRowCtx(ctx, &total, "SELECT COUNT(*) FROM review"+where, args...); err != nil {
		return nil, 0, err
	}
	queryArgs := append(append([]any{}, args...), size, (page-1)*size)
	var list []Review
	err := db.QueryRowsCtx(ctx, &list, `SELECT id,review_no,user_id,target_type,target_id,master_code,rating,content,COALESCE(images,'[]') images,status,DATE_FORMAT(create_time,'%Y-%m-%d %H:%i:%s') create_time FROM review`+where+` ORDER BY id DESC LIMIT ? OFFSET ?`, queryArgs...)
	return list, total, err
}

func FindReviewByID(ctx context.Context, id int64) (Review, error) {
	var review Review
	err := db.QueryRowCtx(ctx, &review, `SELECT id,review_no,user_id,target_type,target_id,master_code,rating,content,COALESCE(images,'[]') images,status,DATE_FORMAT(create_time,'%Y-%m-%d %H:%i:%s') create_time FROM review WHERE id=?`, id)
	return review, err
}

func CreateReview(ctx context.Context, review Review) (Review, error) {
	if review.Status == "" {
		review.Status = ReviewStatusNormal
	}
	random := make([]byte, 5)
	if _, err := rand.Read(random); err != nil {
		return Review{}, err
	}
	reviewNo := fmt.Sprintf("R%s%s", time.Now().Format("20060102150405"), hex.EncodeToString(random))
	result, err := db.ExecCtx(ctx, `INSERT INTO review(review_no,user_id,target_type,target_id,master_code,rating,content,images,status) VALUES(?,?,?,?,?,?,?,?,?)`, reviewNo, review.UserId, review.TargetType, review.TargetId, review.MasterCode, review.Rating, review.Content, review.Images, review.Status)
	if err != nil {
		return Review{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Review{}, err
	}
	return FindReviewByID(ctx, id)
}

func UpsertBookingReview(ctx context.Context, bookingId, userId, masterCode string, rating int, content, images string) error {
	digest := sha256.Sum256([]byte(bookingId))
	reviewNo := "BR" + hex.EncodeToString(digest[:10])
	_, err := db.ExecCtx(ctx, `INSERT INTO review(review_no,user_id,target_type,target_id,master_code,rating,content,images,status)
		VALUES(?,?,?,?,?,?,?,?,?) ON DUPLICATE KEY UPDATE master_code=VALUES(master_code),rating=VALUES(rating),content=VALUES(content),images=VALUES(images),status=VALUES(status)`,
		reviewNo, userId, TargetTypeBooking, bookingId, masterCode, rating, content, images, ReviewStatusNormal)
	return err
}

func UpdateReviewStatus(ctx context.Context, id int64, status string) (bool, error) {
	result, err := db.ExecCtx(ctx, "UPDATE review SET status=? WHERE id=?", status, id)
	if err != nil {
		return false, err
	}
	rows, err := result.RowsAffected()
	return rows == 1, err
}

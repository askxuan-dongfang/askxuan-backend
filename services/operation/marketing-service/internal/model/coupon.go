package model

import (
	"context"
	"fmt"
	"time"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const (
	CouponTypeFullReduce = "full_reduce"
	CouponTypeDiscount   = "discount"
	CouponTypeNewUser    = "new_user"
	CouponTypeCategory   = "category"
)

type Coupon struct {
	Id            int64   `db:"id" json:"id"`
	CouponNo      string  `db:"coupon_no" json:"couponNo"`
	Name          string  `db:"name" json:"name"`
	Type          string  `db:"type" json:"type"`
	Value         float64 `db:"value" json:"value"`
	MinAmount     float64 `db:"min_amount" json:"minAmount"`
	CategoryId    string  `db:"category_id" json:"categoryId"`
	StartTime     string  `db:"start_time" json:"startTime"`
	EndTime       string  `db:"end_time" json:"endTime"`
	TotalCount    int     `db:"total_count" json:"totalCount"`
	ReceivedCount int     `db:"received_count" json:"receivedCount"`
	Status        string  `db:"status" json:"status"`
	CreatedAt     string  `db:"created_at" json:"createdAt"`
}

const couponColumns = `id,coupon_no,name,type,value,min_amount,category_id,IFNULL(DATE_FORMAT(start_time,'%Y-%m-%d %H:%i:%s'),'') start_time,IFNULL(DATE_FORMAT(end_time,'%Y-%m-%d %H:%i:%s'),'') end_time,total_count,received_count,status,DATE_FORMAT(create_time,'%Y-%m-%d %H:%i:%s') created_at`

func FindCouponByID(id int64) (Coupon, bool) {
	var c Coupon
	if err := db.QueryRowCtx(context.Background(), &c, `SELECT `+couponColumns+` FROM coupon WHERE id=?`, id); err != nil {
		return Coupon{}, false
	}
	return c, true
}

func ListCoupons(status, cType string, page, size int) ([]Coupon, int64) {
	where, args := "1=1", []interface{}{}
	if status != "" {
		where += " AND status=?"
		args = append(args, status)
	}
	if cType != "" {
		where += " AND type=?"
		args = append(args, cType)
	}
	var total int64
	if db.QueryRowCtx(context.Background(), &total, "SELECT COUNT(1) FROM coupon WHERE "+where, args...) != nil {
		return []Coupon{}, 0
	}
	offset, limit := pageArgs(page, size)
	var list []Coupon
	if db.QueryRowsCtx(context.Background(), &list, `SELECT `+couponColumns+` FROM coupon WHERE `+where+` ORDER BY id DESC LIMIT ?,?`, append(args, offset, limit)...) != nil {
		return []Coupon{}, 0
	}
	return list, total
}

func InsertCoupon(c Coupon) Coupon {
	if c.Status == "" {
		c.Status = StatusEnabled
	}
	c.CouponNo = fmt.Sprintf("C%s%05d", time.Now().Format("20060102150405"), time.Now().UnixNano()%100000)
	res, err := db.ExecCtx(context.Background(), `INSERT INTO coupon(coupon_no,name,type,value,min_amount,category_id,start_time,end_time,total_count,received_count,status) VALUES(?,?,?,?,?,?,NULLIF(?,''),NULLIF(?,''),?,0,?)`, c.CouponNo, c.Name, c.Type, c.Value, c.MinAmount, c.CategoryId, c.StartTime, c.EndTime, c.TotalCount, c.Status)
	if err != nil {
		return Coupon{}
	}
	c.Id, _ = res.LastInsertId()
	c.CreatedAt = nowStr()
	return c
}

func UpdateCoupon(id int64, c Coupon) (Coupon, bool) {
	res, err := db.ExecCtx(context.Background(), `UPDATE coupon SET name=COALESCE(NULLIF(?,''),name),type=COALESCE(NULLIF(?,''),type),value=?,min_amount=?,category_id=?,start_time=COALESCE(NULLIF(?,''),start_time),end_time=COALESCE(NULLIF(?,''),end_time),total_count=?,status=COALESCE(NULLIF(?,''),status) WHERE id=?`, c.Name, c.Type, c.Value, c.MinAmount, c.CategoryId, c.StartTime, c.EndTime, c.TotalCount, c.Status, id)
	if err != nil {
		return Coupon{}, false
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return Coupon{}, false
	}
	return FindCouponByID(id)
}

func findCouponForUpdate(ctx context.Context, session sqlx.Session, id int64) (Coupon, error) {
	var c Coupon
	err := session.QueryRowCtx(ctx, &c, `SELECT `+couponColumns+` FROM coupon WHERE id=? FOR UPDATE`, id)
	return c, err
}

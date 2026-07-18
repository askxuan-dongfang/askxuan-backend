package model

import (
	"context"
	"time"
)

const (
	StatusEnabled     = "enabled"
	StatusDisabled    = "disabled"
	LinkTypeTemple    = "temple"
	LinkTypeMaster    = "master"
	LinkTypeProduct   = "product"
	LinkTypeDiy       = "diy"
	LinkTypeAdLanding = "ad_landing"
)

type Banner struct {
	Id        int64  `db:"id" json:"id"`
	Title     string `db:"title" json:"title"`
	ImageUrl  string `db:"image_url" json:"imageUrl"`
	LinkType  string `db:"link_type" json:"linkType"`
	LinkValue string `db:"link_value" json:"linkValue"`
	Sort      int    `db:"sort" json:"sort"`
	Status    string `db:"status" json:"status"`
	StartTime string `db:"start_time" json:"startTime"`
	EndTime   string `db:"end_time" json:"endTime"`
	CreatedAt string `db:"created_at" json:"createdAt"`
}

func nowStr() string { return time.Now().Format("2006-01-02 15:04:05") }

func ListBanners(status string, page, size int) ([]Banner, int64) {
	where, args := "1=1", []interface{}{}
	if status != "" {
		where += " AND status=?"
		args = append(args, status)
	}
	var total int64
	if err := db.QueryRowCtx(context.Background(), &total, "SELECT COUNT(1) FROM banner WHERE "+where, args...); err != nil {
		return []Banner{}, 0
	}
	offset, limit := pageArgs(page, size)
	query := `SELECT id,title,image_url,link_type,link_value,sort,status,IFNULL(DATE_FORMAT(start_time,'%Y-%m-%d %H:%i:%s'), '') start_time,IFNULL(DATE_FORMAT(end_time,'%Y-%m-%d %H:%i:%s'), '') end_time,DATE_FORMAT(create_time,'%Y-%m-%d %H:%i:%s') created_at FROM banner WHERE ` + where + ` ORDER BY sort,id DESC LIMIT ?,?`
	var list []Banner
	if err := db.QueryRowsCtx(context.Background(), &list, query, append(args, offset, limit)...); err != nil {
		return []Banner{}, 0
	}
	return list, total
}

func InsertBanner(b Banner) Banner {
	if b.Status == "" {
		b.Status = StatusEnabled
	}
	res, err := db.ExecCtx(context.Background(), `INSERT INTO banner(title,image_url,link_type,link_value,sort,status,start_time,end_time) VALUES(?,?,?,?,?,?,NULLIF(?,''),NULLIF(?,''))`, b.Title, b.ImageUrl, b.LinkType, b.LinkValue, b.Sort, b.Status, b.StartTime, b.EndTime)
	if err != nil {
		return Banner{}
	}
	b.Id, _ = res.LastInsertId()
	b.CreatedAt = nowStr()
	return b
}

func UpdateBanner(id int64, b Banner) (Banner, bool) {
	res, err := db.ExecCtx(context.Background(), `UPDATE banner SET title=COALESCE(NULLIF(?,''),title),image_url=COALESCE(NULLIF(?,''),image_url),link_type=COALESCE(NULLIF(?,''),link_type),link_value=COALESCE(NULLIF(?,''),link_value),sort=?,status=COALESCE(NULLIF(?,''),status),start_time=COALESCE(NULLIF(?,''),start_time),end_time=COALESCE(NULLIF(?,''),end_time) WHERE id=?`, b.Title, b.ImageUrl, b.LinkType, b.LinkValue, b.Sort, b.Status, b.StartTime, b.EndTime, id)
	if err != nil {
		return Banner{}, false
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return Banner{}, false
	}
	list, _ := ListBanners("", 1, 100)
	for _, item := range list {
		if item.Id == id {
			return item, true
		}
	}
	return Banner{}, false
}

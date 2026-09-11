package model

import (
	"context"
	"github.com/askxuan/marketing-service/internal/types"
	"time"
)

const (
	StatusEnabled       = "enabled"
	StatusDisabled      = "disabled"
	StatusDraft         = "draft"
	BannerPlacementHome = "customer_home"
	LinkTypeTemple      = "temple"
	LinkTypeMaster      = "master"
	LinkTypeProduct     = "product"
	LinkTypeDiy         = "diy"
	LinkTypeAdLanding   = "ad_landing"
)

type Banner struct {
	Id        int64  `db:"id" json:"id"`
	Title     string `db:"title" json:"title"`
	Placement string `db:"placement" json:"placement"`
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

const bannerColumns = `id,title,placement,image_url,link_type,link_value,sort,status,IFNULL(DATE_FORMAT(start_time,'%Y-%m-%d %H:%i:%s'),'') start_time,IFNULL(DATE_FORMAT(end_time,'%Y-%m-%d %H:%i:%s'),'') end_time,DATE_FORMAT(create_time,'%Y-%m-%d %H:%i:%s') created_at`

// Availability is applied before COUNT and LIMIT; expired rows never consume a page.
func ListBanners(ctx context.Context, status, placement string, page, size int, activeAt string) ([]Banner, int64, error) {
	where, args := "1=1", []interface{}{}
	if status != "" {
		where += " AND status=?"
		args = append(args, status)
	}
	if placement != "" {
		where += " AND placement=?"
		args = append(args, placement)
	}
	if activeAt != "" {
		where += " AND (start_time IS NULL OR start_time<=?) AND (end_time IS NULL OR end_time>=?)"
		args = append(args, activeAt, activeAt)
	}
	var total int64
	if err := db.QueryRowCtx(ctx, &total, "SELECT COUNT(1) FROM banner WHERE "+where, args...); err != nil {
		return nil, 0, err
	}
	offset, limit := pageArgs(page, size)
	list := make([]Banner, 0)
	err := db.QueryRowsCtx(ctx, &list, "SELECT "+bannerColumns+" FROM banner WHERE "+where+" ORDER BY sort,id DESC LIMIT ?,?", append(args, offset, limit)...)
	return list, total, err
}
func FindBanner(ctx context.Context, id int64) (Banner, error) {
	var b Banner
	err := db.QueryRowCtx(ctx, &b, "SELECT "+bannerColumns+" FROM banner WHERE id=?", id)
	return b, err
}
func InsertBanner(ctx context.Context, b Banner) (Banner, error) {
	b.Status = StatusDraft // Publishing is a separate validated action.
	res, err := db.ExecCtx(ctx, `INSERT INTO banner(title,placement,image_url,link_type,link_value,sort,status,start_time,end_time) VALUES(?,?,?,?,?,?,?,NULLIF(?,''),NULLIF(?,''))`, b.Title, b.Placement, b.ImageUrl, b.LinkType, b.LinkValue, b.Sort, b.Status, b.StartTime, b.EndTime)
	if err != nil {
		return Banner{}, err
	}
	b.Id, err = res.LastInsertId()
	b.CreatedAt = nowStr()
	return b, err
}
func UpdateBanner(ctx context.Context, id int64, p types.BannerUpdateReq) (Banner, error) {
	_, err := db.ExecCtx(ctx, `UPDATE banner SET title=COALESCE(?,title),placement=COALESCE(?,placement),image_url=COALESCE(?,image_url),link_type=COALESCE(?,link_type),link_value=COALESCE(?,link_value),sort=COALESCE(?,sort),status=COALESCE(?,status),start_time=CASE WHEN ? IS NULL THEN start_time ELSE NULLIF(?,'') END,end_time=CASE WHEN ? IS NULL THEN end_time ELSE NULLIF(?,'') END WHERE id=?`, p.Title, p.Placement, p.ImageUrl, p.LinkType, p.LinkValue, p.Sort, p.Status, p.StartTime, p.StartTime, p.EndTime, p.EndTime, id)
	if err != nil {
		return Banner{}, err
	}
	// An unchanged edit succeeds too; RowsAffected=0 does not mean missing.
	return FindBanner(ctx, id)
}

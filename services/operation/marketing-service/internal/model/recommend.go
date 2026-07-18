package model

import "context"

const (
	RecommendTypeTemple  = "temple"
	RecommendTypeMaster  = "master"
	RecommendTypeProduct = "product"
)

type Recommend struct {
	Id        int64  `db:"id" json:"id"`
	Type      string `db:"type" json:"type"`
	TargetId  string `db:"target_id" json:"targetId"`
	Sort      int    `db:"sort" json:"sort"`
	Status    string `db:"status" json:"status"`
	CreatedAt string `db:"created_at" json:"createdAt"`
}

func ListRecommends(rType, status string, page, size int) ([]Recommend, int64) {
	where, args := "1=1", []interface{}{}
	if rType != "" {
		where += " AND type=?"
		args = append(args, rType)
	}
	if status != "" {
		where += " AND status=?"
		args = append(args, status)
	}
	var total int64
	if db.QueryRowCtx(context.Background(), &total, "SELECT COUNT(1) FROM recommend WHERE "+where, args...) != nil {
		return []Recommend{}, 0
	}
	offset, limit := pageArgs(page, size)
	var list []Recommend
	if db.QueryRowsCtx(context.Background(), &list, `SELECT id,type,target_id,sort,status,DATE_FORMAT(create_time,'%Y-%m-%d %H:%i:%s') created_at FROM recommend WHERE `+where+` ORDER BY sort,id DESC LIMIT ?,?`, append(args, offset, limit)...) != nil {
		return []Recommend{}, 0
	}
	return list, total
}
func UpdateRecommend(id int64, r Recommend) (Recommend, bool) {
	res, err := db.ExecCtx(context.Background(), `UPDATE recommend SET type=COALESCE(NULLIF(?,''),type),target_id=COALESCE(NULLIF(?,''),target_id),sort=?,status=COALESCE(NULLIF(?,''),status) WHERE id=?`, r.Type, r.TargetId, r.Sort, r.Status, id)
	if err != nil {
		return Recommend{}, false
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return Recommend{}, false
	}
	list, _ := ListRecommends("", "", 1, 100)
	for _, v := range list {
		if v.Id == id {
			return v, true
		}
	}
	return Recommend{}, false
}

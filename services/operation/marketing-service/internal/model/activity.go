package model

import "context"

const (
	ActivityTypeLimitedDiscount = "limited_discount"
	ActivityTypeFestival        = "festival"
	ActivityTypeTempleEvent     = "temple_event"
)

type Activity struct {
	Id        int64  `db:"id" json:"id"`
	Name      string `db:"name" json:"name"`
	Type      string `db:"type" json:"type"`
	StartTime string `db:"start_time" json:"startTime"`
	EndTime   string `db:"end_time" json:"endTime"`
	Config    string `db:"config" json:"config"`
	Status    string `db:"status" json:"status"`
	CreatedAt string `db:"created_at" json:"createdAt"`
}

func ListActivities(status, aType string, page, size int) ([]Activity, int64) {
	where, args := "1=1", []interface{}{}
	if status != "" {
		where += " AND status=?"
		args = append(args, status)
	}
	if aType != "" {
		where += " AND type=?"
		args = append(args, aType)
	}
	var total int64
	if db.QueryRowCtx(context.Background(), &total, "SELECT COUNT(1) FROM activity WHERE "+where, args...) != nil {
		return []Activity{}, 0
	}
	offset, limit := pageArgs(page, size)
	query := `SELECT id,name,type,IFNULL(DATE_FORMAT(start_time,'%Y-%m-%d %H:%i:%s'),'') start_time,IFNULL(DATE_FORMAT(end_time,'%Y-%m-%d %H:%i:%s'),'') end_time,IFNULL(config,'') config,status,DATE_FORMAT(create_time,'%Y-%m-%d %H:%i:%s') created_at FROM activity WHERE ` + where + ` ORDER BY id DESC LIMIT ?,?`
	var list []Activity
	if db.QueryRowsCtx(context.Background(), &list, query, append(args, offset, limit)...) != nil {
		return []Activity{}, 0
	}
	return list, total
}

func InsertActivity(a Activity) Activity {
	if a.Status == "" {
		a.Status = StatusEnabled
	}
	res, err := db.ExecCtx(context.Background(), `INSERT INTO activity(name,type,start_time,end_time,config,status) VALUES(?,?,NULLIF(?,''),NULLIF(?,''),?,?)`, a.Name, a.Type, a.StartTime, a.EndTime, a.Config, a.Status)
	if err != nil {
		return Activity{}
	}
	a.Id, _ = res.LastInsertId()
	a.CreatedAt = nowStr()
	return a
}

func UpdateActivity(id int64, a Activity) (Activity, bool) {
	res, err := db.ExecCtx(context.Background(), `UPDATE activity SET name=COALESCE(NULLIF(?,''),name),type=COALESCE(NULLIF(?,''),type),start_time=COALESCE(NULLIF(?,''),start_time),end_time=COALESCE(NULLIF(?,''),end_time),config=COALESCE(NULLIF(?,''),config),status=COALESCE(NULLIF(?,''),status) WHERE id=?`, a.Name, a.Type, a.StartTime, a.EndTime, a.Config, a.Status, id)
	if err != nil {
		return Activity{}, false
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return Activity{}, false
	}
	list, _ := ListActivities("", "", 1, 100)
	for _, v := range list {
		if v.Id == id {
			return v, true
		}
	}
	return Activity{}, false
}

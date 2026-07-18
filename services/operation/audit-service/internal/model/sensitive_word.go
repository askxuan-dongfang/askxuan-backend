package model

import "context"

const (
	SensitiveWordEnabled  = "enabled"
	SensitiveWordDisabled = "disabled"
)

type SensitiveWord struct {
	Id         int64  `db:"id" json:"id"`
	Word       string `db:"word" json:"word"`
	Category   string `db:"category" json:"category"`
	Status     string `db:"status" json:"status"`
	CreateTime string `db:"create_time" json:"createTime"`
}

func ListSensitiveWords(category, status, keyword string, page, size int) ([]SensitiveWord, int64) {
	where, args := "1=1", []interface{}{}
	if category != "" {
		where += " AND category=?"
		args = append(args, category)
	}
	if status != "" {
		where += " AND status=?"
		args = append(args, status)
	}
	if keyword != "" {
		where += " AND word LIKE ?"
		args = append(args, "%"+keyword+"%")
	}
	var total int64
	if db.QueryRowCtx(context.Background(), &total, "SELECT COUNT(1) FROM sensitive_word WHERE "+where, args...) != nil {
		return []SensitiveWord{}, 0
	}
	offset, limit := paging(page, size)
	var list []SensitiveWord
	if db.QueryRowsCtx(context.Background(), &list, `SELECT id,word,category,status,DATE_FORMAT(create_time,'%Y-%m-%d %H:%i:%s') create_time FROM sensitive_word WHERE `+where+` ORDER BY id DESC LIMIT ?,?`, append(args, offset, limit)...) != nil {
		return []SensitiveWord{}, 0
	}
	return list, total
}
func CreateSensitiveWord(word, category string) SensitiveWord {
	res, err := db.ExecCtx(context.Background(), `INSERT INTO sensitive_word(word,category,status) VALUES(?,?,'enabled')`, word, category)
	if err != nil {
		return SensitiveWord{}
	}
	id, _ := res.LastInsertId()
	return SensitiveWord{Id: id, Word: word, Category: category, Status: SensitiveWordEnabled}
}
func DeleteSensitiveWord(id int64) bool {
	res, err := db.ExecCtx(context.Background(), `DELETE FROM sensitive_word WHERE id=?`, id)
	if err != nil {
		return false
	}
	n, _ := res.RowsAffected()
	return n == 1
}

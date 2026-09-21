package logic

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/askxuan/ai-service/internal/experience"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"strings"
	"unicode/utf8"
)

type ExperienceSave struct {
	RequestKey string           `json:"requestKey"`
	Input      experience.Input `json:"input"`
	Favorites  []string         `json:"favorites"`
	Note       string           `json:"note"`
}
type ExperienceNote struct {
	ID        int64           `json:"id" db:"id"`
	Payload   string          `json:"-" db:"payload"`
	CreatedAt string          `json:"createdAt" db:"created_at"`
	Data      *ExperienceData `json:"data" db:"-"`
}
type ExperienceData struct {
	Input     experience.Input  `json:"input"`
	Result    experience.Result `json:"result"`
	Favorites []string          `json:"favorites"`
	Note      string            `json:"note"`
}

func ExperienceSaveNote(ctx context.Context, s *svc.ServiceContext, user string, req ExperienceSave) (*ExperienceNote, error) {
	if len(req.RequestKey) < 8 || len(req.RequestKey) > 64 || utf8.RuneCountInString(req.Note) > 1000 || len(req.Favorites) > 4 {
		return nil, common.ErrParamInvalid
	}
	if req.Favorites == nil {
		req.Favorites = []string{}
	}
	var existing ExperienceNote
	err := s.DB.QueryRowPartialCtx(ctx, &existing, `SELECT id,CAST(payload AS CHAR) payload,created_at FROM ai_experience_note WHERE user_id=? AND request_key=?`, user, req.RequestKey)
	if err == nil {
		return matchExperience(existing, req)
	}
	if !errors.Is(err, sqlx.ErrNotFound) {
		return nil, err
	}
	result, err := experience.Run(req.Input)
	if err != nil {
		return nil, common.NewBizError(40001, err.Error())
	}
	seen := map[string]bool{}
	for _, id := range req.Favorites {
		found := false
		for _, c := range result.Candidates {
			if c.ID == id {
				found = true
			}
		}
		if !found || seen[id] {
			return nil, common.ErrParamInvalid
		}
		seen[id] = true
	}
	data := ExperienceData{req.Input, result, req.Favorites, strings.TrimSpace(req.Note)}
	payload, _ := json.Marshal(data)
	res, err := s.DB.ExecCtx(ctx, `INSERT INTO ai_experience_note(user_id,request_key,skill,version,payload) VALUES(?,?,?,?,?)`, user, req.RequestKey, req.Input.Skill, result.Version, string(payload))
	if err != nil {
		if e := s.DB.QueryRowPartialCtx(ctx, &existing, `SELECT id,CAST(payload AS CHAR) payload,created_at FROM ai_experience_note WHERE user_id=? AND request_key=?`, user, req.RequestKey); e == nil {
			return matchExperience(existing, req)
		}
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return ExperienceGetNote(ctx, s, user, id)
}
func matchExperience(n ExperienceNote, req ExperienceSave) (*ExperienceNote, error) {
	var d ExperienceData
	if json.Unmarshal([]byte(n.Payload), &d) != nil {
		return nil, common.ErrSystem
	}
	// Compare user intent, not computed output: retries survive rule upgrades.
	a, _ := json.Marshal(d.Input)
	b, _ := json.Marshal(req.Input)
	f, _ := json.Marshal(d.Favorites)
	g, _ := json.Marshal(req.Favorites)
	if string(a) != string(b) || string(f) != string(g) || d.Note != strings.TrimSpace(req.Note) {
		return nil, common.NewBizError(40901, "保存内容已改变，请重新保存")
	}
	n.Data = &d
	return &n, nil
}
func ExperienceGetNote(ctx context.Context, s *svc.ServiceContext, user string, id int64) (*ExperienceNote, error) {
	var n ExperienceNote
	if err := s.DB.QueryRowPartialCtx(ctx, &n, `SELECT id,CAST(payload AS CHAR) payload,created_at FROM ai_experience_note WHERE id=? AND user_id=?`, id, user); err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.ErrForbidden
		}
		return nil, err
	}
	if json.Unmarshal([]byte(n.Payload), &n.Data) != nil {
		return nil, common.ErrSystem
	}
	return &n, nil
}
func ExperienceListNotes(ctx context.Context, s *svc.ServiceContext, user string, page int) ([]ExperienceNote, error) {
	if page < 1 {
		page = 1
	}
	if page > 10000 {
		return nil, common.ErrParamInvalid
	}
	notes := make([]ExperienceNote, 0)
	if err := s.DB.QueryRowsPartialCtx(ctx, &notes, `SELECT id,CAST(payload AS CHAR) payload,created_at FROM ai_experience_note WHERE user_id=? ORDER BY id DESC LIMIT 20 OFFSET ?`, user, (page-1)*20); err != nil {
		return nil, err
	}
	for i := range notes {
		if json.Unmarshal([]byte(notes[i].Payload), &notes[i].Data) != nil {
			return nil, common.ErrSystem
		}
	}
	return notes, nil
}
func ExperienceDeleteNote(ctx context.Context, s *svc.ServiceContext, user string, id int64) error {
	_, err := s.DB.ExecCtx(ctx, `DELETE FROM ai_experience_note WHERE id=? AND user_id=?`, id, user)
	return err
}

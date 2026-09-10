package model

import (
	"context"
	"strings"

	"github.com/askxuan/community-service/internal/types"
)

func feedWhere(req *types.FeedReq) (string, []interface{}) {
	where := " WHERE p.status='approved'"
	args := []interface{}{}
	for _, f := range []struct{ value, clause string }{{req.Type, " AND p.type=?"}, {req.BeliefCode, " AND p.belief_code=?"}} {
		if f.value != "" {
			where += f.clause
			args = append(args, f.value)
		}
	}
	if req.Following {
		where += " AND EXISTS (SELECT 1 FROM master_follow f WHERE f.master_id=p.master_id AND f.user_id=?)"
		args = append(args, req.Viewer)
	}
	if req.Keyword != "" {
		keyword := "%" + strings.NewReplacer("!", "!!", "%", "!%", "_", "!_").Replace(req.Keyword) + "%"
		where += " AND (p.title LIKE ? ESCAPE '!' OR p.content LIKE ? ESCAPE '!')"
		args = append(args, keyword, keyword)
	}
	return where, args
}

// Filtering takes place before COUNT/LIMIT so a following feed can span all pages.
func (m *communityModel) ListFeed(ctx context.Context, req *types.FeedReq, page, size int) (int64, []types.Post, error) {
	_, size, offset := pageArgs(page, size)
	where, args := feedWhere(req)
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, "SELECT COUNT(*) FROM post p"+where, args...); err != nil {
		return 0, nil, err
	}
	order := "p.create_time DESC,p.post_no DESC"
	if req.Sort == "popular" {
		order = "p.like_count DESC,p.comment_count DESC," + order
	}
	var rows []postRow
	if err := m.conn.QueryRowsCtx(ctx, &rows, "SELECT p."+strings.ReplaceAll(postRows, ",", ",p.")+" FROM post p"+where+" ORDER BY "+order+" LIMIT ? OFFSET ?", append(args, size, offset)...); err != nil {
		return 0, nil, err
	}
	posts := make([]types.Post, 0, len(rows))
	if len(rows) == 0 {
		return total, posts, nil
	}
	ids := make([]interface{}, 0, len(rows))
	marks := make([]string, 0, len(rows))
	for i := range rows {
		posts = append(posts, rowToPost(&rows[i]))
		ids = append(ids, rows[i].PostNo)
		marks = append(marks, "?")
	}
	var assets []struct {
		PostNo    string `db:"post_no"`
		Id        int64  `db:"id"`
		MediaId   int64  `db:"media_id"`
		AssetType string `db:"asset_type"`
		Sort      int    `db:"sort"`
	}
	if err := m.conn.QueryRowsCtx(ctx, &assets, "SELECT post_no,id,media_id,asset_type,sort FROM post_asset WHERE post_no IN ("+strings.Join(marks, ",")+") ORDER BY sort,id", ids...); err != nil {
		return 0, nil, err
	}
	byPost := map[string][]types.Asset{}
	for _, a := range assets {
		byPost[a.PostNo] = append(byPost[a.PostNo], types.Asset{Id: a.Id, MediaId: a.MediaId, AssetType: a.AssetType, Sort: a.Sort})
	}
	liked := map[string]bool{}
	followed := map[string]bool{}
	if req.Viewer != "" {
		var likes []string
		if err := m.conn.QueryRowsCtx(ctx, &likes, "SELECT post_no FROM post_like WHERE user_id=? AND post_no IN ("+strings.Join(marks, ",")+")", append([]interface{}{req.Viewer}, ids...)...); err != nil {
			return 0, nil, err
		}
		for _, id := range likes {
			liked[id] = true
		}
		var follows []string
		if err := m.conn.QueryRowsCtx(ctx, &follows, "SELECT DISTINCT f.master_id FROM master_follow f JOIN post p ON p.master_id=f.master_id WHERE f.user_id=? AND p.post_no IN ("+strings.Join(marks, ",")+")", append([]interface{}{req.Viewer}, ids...)...); err != nil {
			return 0, nil, err
		}
		for _, id := range follows {
			followed[id] = true
		}
	}
	for i := range posts {
		if a := byPost[posts[i].Id]; a != nil {
			posts[i].Assets = a
		}
		posts[i].Liked = liked[posts[i].Id]
		posts[i].Following = followed[posts[i].MasterId]
	}
	return total, posts, nil
}

// Batch only media visible to this viewer. Review bypass is used solely by the role-checked admin list.
func (m *communityModel) EnrichMedia(ctx context.Context, posts []types.Post, viewer string, review bool) error {
	if viewer == "" && !review {
		return nil
	}
	seen := map[int64]bool{}
	ids := []interface{}{}
	marks := []string{}
	add := func(id int64) {
		if id > 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
			marks = append(marks, "?")
		}
	}
	for _, p := range posts {
		add(p.CoverMediaId)
		for _, a := range p.Assets {
			add(a.MediaId)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	query := "SELECT id,playback_url,cover_url,duration FROM askxuan_media.media_asset WHERE id IN (" + strings.Join(marks, ",") + ") AND status='ready'"
	if !review {
		query += " AND (audit_status='approved' OR owner_id=?)"
		ids = append(ids, viewer)
	}
	var rows []struct {
		Id       int64   `db:"id"`
		Url      string  `db:"playback_url"`
		CoverUrl string  `db:"cover_url"`
		Duration float64 `db:"duration"`
	}
	if err := m.conn.QueryRowsCtx(ctx, &rows, query, ids...); err != nil {
		return err
	}
	for i := range posts {
		for _, r := range rows {
			if posts[i].CoverMediaId == r.Id {
				posts[i].CoverUrl = r.Url
			}
			for j := range posts[i].Assets {
				a := &posts[i].Assets[j]
				if a.MediaId == r.Id {
					a.Url = r.Url
					a.CoverUrl = r.CoverUrl
					a.Duration = r.Duration
				}
			}
		}
	}
	return nil
}

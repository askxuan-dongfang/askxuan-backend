package model

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/askxuan/community-service/internal/types"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

const postRows = "post_no,master_id,owner_id,type,title,content,cover_media_id,belief_code,status,audit_id,audit_remark,like_count,comment_count,create_time,update_time"

type postRow struct {
	PostNo       string `db:"post_no"`
	MasterId     string `db:"master_id"`
	OwnerId      string `db:"owner_id"`
	Type         string `db:"type"`
	Title        string `db:"title"`
	Content      string `db:"content"`
	CoverMediaId int64  `db:"cover_media_id"`
	BeliefCode   string `db:"belief_code"`
	Status       string `db:"status"`
	AuditId      int64  `db:"audit_id"`
	AuditRemark  string `db:"audit_remark"`
	LikeCount    int64  `db:"like_count"`
	CommentCount int64  `db:"comment_count"`
	CreateTime   string `db:"create_time"`
	UpdateTime   string `db:"update_time"`
}

type commentRow struct {
	CommentNo   string `db:"comment_no"`
	PostNo      string `db:"post_no"`
	UserId      string `db:"user_id"`
	Content     string `db:"content"`
	Status      string `db:"status"`
	AuditId     int64  `db:"audit_id"`
	AuditRemark string `db:"audit_remark"`
	CreateTime  string `db:"create_time"`
}

type CommunityModel interface {
	ValidateAssets(ctx context.Context, ownerId string, coverMediaId int64, assets []types.Asset) error
	ListPosts(ctx context.Context, status, postType, beliefCode, ownerId string, page, size int) (int64, []types.Post, error)
	FindPost(ctx context.Context, postNo, viewer string, ownerView bool) (*types.Post, error)
	CreatePost(ctx context.Context, req *types.PostWriteReq) (*types.Post, error)
	UpdatePost(ctx context.Context, req *types.PostWriteReq) (*types.Post, error)
	ChangePostStatus(ctx context.Context, postNo, ownerId, status string) (*types.Post, error)
	ReviewPost(ctx context.Context, postNo, auditorId, status, remark string) (*types.Post, error)
	SetLike(ctx context.Context, postNo, userId string, liked bool) (*types.LikeResp, error)
	CreateComment(ctx context.Context, postNo, userId, content string) (*types.Comment, error)
	ListComments(ctx context.Context, postNo, status string, page, size int) (int64, []types.Comment, error)
	ReviewComment(ctx context.Context, commentNo, auditorId, status, remark string) (*types.Comment, error)
	SetFollow(ctx context.Context, masterId, userId string, following bool) (bool, error)
}

type communityModel struct{ conn sqlx.SqlConn }

func NewCommunityModel(conn sqlx.SqlConn) CommunityModel { return &communityModel{conn: conn} }

func pageArgs(page, size int) (int, int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 50 {
		size = 20
	}
	return page, size, (page - 1) * size
}

func (m *communityModel) ValidateAssets(ctx context.Context, ownerId string, coverMediaId int64, assets []types.Asset) error {
	for _, asset := range assets {
		var count int64
		if err := m.conn.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM askxuan_media.media_asset WHERE id=? AND owner_id=? AND media_type=? AND status='ready'", asset.MediaId, ownerId, asset.AssetType); err != nil || count != 1 {
			if err != nil {
				return err
			}
			return sqlx.ErrNotFound
		}
	}
	if coverMediaId > 0 {
		var count int64
		if err := m.conn.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM askxuan_media.media_asset WHERE id=? AND owner_id=? AND media_type='image' AND status='ready'", coverMediaId, ownerId); err != nil || count != 1 {
			if err != nil {
				return err
			}
			return sqlx.ErrNotFound
		}
	}
	return nil
}

func (m *communityModel) ListPosts(ctx context.Context, status, postType, beliefCode, ownerId string, page, size int) (int64, []types.Post, error) {
	page, size, offset := pageArgs(page, size)
	where := " WHERE 1=1"
	args := make([]interface{}, 0)
	for _, filter := range []struct{ value, clause string }{{status, " AND status=?"}, {postType, " AND type=?"}, {beliefCode, " AND belief_code=?"}, {ownerId, " AND owner_id=?"}} {
		if filter.value != "" {
			where += filter.clause
			args = append(args, filter.value)
		}
	}
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, "SELECT COUNT(*) FROM post"+where, args...); err != nil {
		return 0, nil, err
	}
	var rows []postRow
	queryArgs := append(append([]interface{}{}, args...), size, offset)
	if err := m.conn.QueryRowsCtx(ctx, &rows, "SELECT "+postRows+" FROM post"+where+" ORDER BY create_time DESC LIMIT ? OFFSET ?", queryArgs...); err != nil {
		return 0, nil, err
	}
	posts := make([]types.Post, 0, len(rows))
	for i := range rows {
		p := rowToPost(&rows[i])
		assets, err := m.findAssets(ctx, p.Id)
		if err != nil {
			return 0, nil, err
		}
		p.Assets = assets
		posts = append(posts, p)
	}
	_ = page
	return total, posts, nil
}

func (m *communityModel) FindPost(ctx context.Context, postNo, viewer string, ownerView bool) (*types.Post, error) {
	var row postRow
	query := "SELECT " + postRows + " FROM post WHERE post_no=?"
	args := []interface{}{postNo}
	if ownerView {
		query += " AND owner_id=?"
		args = append(args, viewer)
	} else {
		query += " AND status='approved'"
	}
	if err := m.conn.QueryRowCtx(ctx, &row, query, args...); err != nil {
		return nil, err
	}
	p := rowToPost(&row)
	assets, err := m.findAssets(ctx, postNo)
	if err != nil {
		return nil, err
	}
	p.Assets = assets
	if viewer != "" {
		var count int64
		_ = m.conn.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM post_like WHERE post_no=? AND user_id=?", postNo, viewer)
		p.Liked = count > 0
	}
	return &p, nil
}

func (m *communityModel) findAnyPost(ctx context.Context, postNo string) (*types.Post, error) {
	var row postRow
	if err := m.conn.QueryRowCtx(ctx, &row, "SELECT "+postRows+" FROM post WHERE post_no=?", postNo); err != nil {
		return nil, err
	}
	p := rowToPost(&row)
	assets, err := m.findAssets(ctx, postNo)
	if err != nil {
		return nil, err
	}
	p.Assets = assets
	return &p, nil
}

func (m *communityModel) findAssets(ctx context.Context, postNo string) ([]types.Asset, error) {
	var assets []types.Asset
	err := m.conn.QueryRowsCtx(ctx, &assets, "SELECT id,media_id,asset_type,sort FROM post_asset WHERE post_no=? ORDER BY sort,id", postNo)
	if assets == nil {
		assets = []types.Asset{}
	}
	return assets, err
}

func (m *communityModel) CreatePost(ctx context.Context, req *types.PostWriteReq) (*types.Post, error) {
	postNo := fmt.Sprintf("P%d", time.Now().UnixNano())
	status := "draft"
	if req.Submit {
		status = "pending"
	}
	err := m.conn.TransactCtx(ctx, func(_ context.Context, s sqlx.Session) error {
		_, err := s.ExecCtx(ctx, "INSERT INTO post(post_no,master_id,owner_id,type,title,content,cover_media_id,belief_code,status) VALUES(?,?,?,?,?,?,?,?,?)", postNo, req.MasterId, req.OwnerId, req.Type, req.Title, req.Content, req.CoverMediaId, req.BeliefCode, status)
		if err != nil {
			return err
		}
		if err = replaceAssets(ctx, s, postNo, req.Assets); err != nil {
			return err
		}
		if status == "pending" {
			return submitAudit(ctx, s, "post", postNo, req.OwnerId, req)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return m.FindPost(ctx, postNo, req.OwnerId, true)
}

func (m *communityModel) UpdatePost(ctx context.Context, req *types.PostWriteReq) (*types.Post, error) {
	err := m.conn.TransactCtx(ctx, func(_ context.Context, s sqlx.Session) error {
		status := "draft"
		if req.Submit {
			status = "pending"
		}
		r, err := s.ExecCtx(ctx, "UPDATE post SET type=?,title=?,content=?,cover_media_id=?,belief_code=?,status=?,audit_remark='',update_time=CURRENT_TIMESTAMP WHERE post_no=? AND owner_id=? AND status IN ('draft','rejected')", req.Type, req.Title, req.Content, req.CoverMediaId, req.BeliefCode, status, req.Id, req.OwnerId)
		if err != nil {
			return err
		}
		if n, _ := r.RowsAffected(); n != 1 {
			return sqlx.ErrNotFound
		}
		if err = replaceAssets(ctx, s, req.Id, req.Assets); err != nil {
			return err
		}
		if req.Submit {
			return submitAudit(ctx, s, "post", req.Id, req.OwnerId, req)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return m.FindPost(ctx, req.Id, req.OwnerId, true)
}

func replaceAssets(ctx context.Context, s sqlx.Session, postNo string, assets []types.Asset) error {
	if _, err := s.ExecCtx(ctx, "DELETE FROM post_asset WHERE post_no=?", postNo); err != nil {
		return err
	}
	for _, a := range assets {
		if _, err := s.ExecCtx(ctx, "INSERT INTO post_asset(post_no,media_id,asset_type,sort) VALUES(?,?,?,?)", postNo, a.MediaId, a.AssetType, a.Sort); err != nil {
			return err
		}
	}
	return nil
}

func submitAudit(ctx context.Context, s sqlx.Session, bizType, bizId, submitter string, snapshot interface{}) error {
	b, _ := json.Marshal(snapshot)
	r, err := s.ExecCtx(ctx, "INSERT INTO askxuan_audit.audit_queue(biz_type,biz_id,submitter_id,content_snapshot,status) VALUES(?,?,?,?, 'pending')", bizType, bizId, submitter, string(b))
	if err != nil {
		return err
	}
	auditId, err := r.LastInsertId()
	if err != nil {
		return err
	}
	table := "post"
	idField := "post_no"
	if bizType == "comment" {
		table = "post_comment"
		idField = "comment_no"
	}
	_, err = s.ExecCtx(ctx, "UPDATE "+table+" SET audit_id=? WHERE "+idField+"=?", auditId, bizId)
	return err
}

func (m *communityModel) ChangePostStatus(ctx context.Context, postNo, ownerId, status string) (*types.Post, error) {
	if status == "submit" {
		err := m.conn.TransactCtx(ctx, func(_ context.Context, s sqlx.Session) error {
			r, e := s.ExecCtx(ctx, "UPDATE post SET status='pending',audit_remark='',update_time=CURRENT_TIMESTAMP WHERE post_no=? AND owner_id=? AND status IN ('draft','rejected')", postNo, ownerId)
			if e != nil {
				return e
			}
			if n, _ := r.RowsAffected(); n != 1 {
				return sqlx.ErrNotFound
			}
			return submitAudit(ctx, s, "post", postNo, ownerId, map[string]string{"postId": postNo})
		})
		if err != nil {
			return nil, err
		}
	} else {
		r, err := m.conn.ExecCtx(ctx, "UPDATE post SET status=?,update_time=CURRENT_TIMESTAMP WHERE post_no=? AND owner_id=? AND status='approved'", status, postNo, ownerId)
		if err != nil {
			return nil, err
		}
		if n, _ := r.RowsAffected(); n != 1 {
			return nil, sqlx.ErrNotFound
		}
	}
	return m.FindPost(ctx, postNo, ownerId, true)
}

func (m *communityModel) ReviewPost(ctx context.Context, postNo, auditorId, status, remark string) (*types.Post, error) {
	err := m.conn.TransactCtx(ctx, func(_ context.Context, s sqlx.Session) error {
		return review(ctx, s, "post", "post_no", postNo, auditorId, status, remark, false)
	})
	if err != nil {
		return nil, err
	}
	return m.findAnyPost(ctx, postNo)
}

func review(ctx context.Context, s sqlx.Session, table, idField, id, auditor, status, remark string, comment bool) error {
	var current struct {
		Status  string `db:"status"`
		AuditId int64  `db:"audit_id"`
	}
	if comment {
		var commentCurrent struct {
			Status  string `db:"status"`
			AuditId int64  `db:"audit_id"`
			PostNo  string `db:"post_no"`
		}
		if err := s.QueryRowCtx(ctx, &commentCurrent, "SELECT status,audit_id,post_no FROM "+table+" WHERE "+idField+"=? FOR UPDATE", id); err != nil {
			return err
		}
		if commentCurrent.Status == status {
			return nil
		}
		if commentCurrent.Status != "pending" {
			return sqlx.ErrNotFound
		}
		current.Status, current.AuditId = commentCurrent.Status, commentCurrent.AuditId
		if err := applyReview(ctx, s, table, idField, id, auditor, status, remark, current.Status, current.AuditId); err != nil {
			return err
		}
		if status == "approved" {
			_, err := s.ExecCtx(ctx, "UPDATE post SET comment_count=comment_count+1 WHERE post_no=?", commentCurrent.PostNo)
			return err
		}
		return nil
	}
	if err := s.QueryRowCtx(ctx, &current, "SELECT status,audit_id FROM "+table+" WHERE "+idField+"=? FOR UPDATE", id); err != nil {
		return err
	}
	return applyReview(ctx, s, table, idField, id, auditor, status, remark, current.Status, current.AuditId)
}

func applyReview(ctx context.Context, s sqlx.Session, table, idField, id, auditor, status, remark, currentStatus string, auditId int64) error {
	if currentStatus == status {
		return nil
	}
	if currentStatus != "pending" {
		return sqlx.ErrNotFound
	}
	if _, err := s.ExecCtx(ctx, "UPDATE "+table+" SET status=?,audit_remark=?,update_time=CURRENT_TIMESTAMP WHERE "+idField+"=?", status, remark, id); err != nil {
		return err
	}
	r, err := s.ExecCtx(ctx, "UPDATE askxuan_audit.audit_queue SET status=?,auditor_id=?,audit_time=CURRENT_TIMESTAMP,audit_remark=?,update_time=CURRENT_TIMESTAMP WHERE id=? AND status='pending'", status, auditor, remark, auditId)
	if err != nil {
		return err
	}
	if n, _ := r.RowsAffected(); n != 1 {
		return sqlx.ErrNotFound
	}
	if _, err := s.ExecCtx(ctx, "INSERT INTO askxuan_audit.audit_log(audit_id,action,operator_id,remark) VALUES(?,?,?,?)", auditId, status, auditor, remark); err != nil {
		return err
	}
	return nil
}

func (m *communityModel) SetLike(ctx context.Context, postNo, userId string, liked bool) (*types.LikeResp, error) {
	err := m.conn.TransactCtx(ctx, func(_ context.Context, s sqlx.Session) error {
		var r sql.Result
		var e error
		if liked {
			r, e = s.ExecCtx(ctx, "INSERT IGNORE INTO post_like(post_no,user_id) SELECT post_no,? FROM post WHERE post_no=? AND status='approved'", userId, postNo)
		} else {
			r, e = s.ExecCtx(ctx, "DELETE FROM post_like WHERE post_no=? AND user_id=?", postNo, userId)
		}
		if e != nil {
			return e
		}
		if n, _ := r.RowsAffected(); n > 0 {
			delta := -1
			if liked {
				delta = 1
			}
			_, e = s.ExecCtx(ctx, "UPDATE post SET like_count=GREATEST(0,like_count+?) WHERE post_no=?", delta, postNo)
		}
		return e
	})
	if err != nil {
		return nil, err
	}
	var count int64
	if err = m.conn.QueryRowCtx(ctx, &count, "SELECT like_count FROM post WHERE post_no=? AND status='approved'", postNo); err != nil {
		return nil, err
	}
	return &types.LikeResp{Liked: liked, LikeCount: count}, nil
}

func (m *communityModel) CreateComment(ctx context.Context, postNo, userId, content string) (*types.Comment, error) {
	commentNo := fmt.Sprintf("C%d", time.Now().UnixNano())
	err := m.conn.TransactCtx(ctx, func(_ context.Context, s sqlx.Session) error {
		var n int64
		if e := s.QueryRowCtx(ctx, &n, "SELECT COUNT(*) FROM post WHERE post_no=? AND status='approved'", postNo); e != nil || n != 1 {
			if e != nil {
				return e
			}
			return sqlx.ErrNotFound
		}
		if _, e := s.ExecCtx(ctx, "INSERT INTO post_comment(comment_no,post_no,user_id,content,status) VALUES(?,?,?,?, 'pending')", commentNo, postNo, userId, content); e != nil {
			return e
		}
		return submitAudit(ctx, s, "comment", commentNo, userId, map[string]string{"postId": postNo, "content": content})
	})
	if err != nil {
		return nil, err
	}
	var row commentRow
	if err = m.conn.QueryRowCtx(ctx, &row, "SELECT comment_no,post_no,user_id,content,status,audit_id,audit_remark,create_time FROM post_comment WHERE comment_no=?", commentNo); err != nil {
		return nil, err
	}
	c := rowToComment(&row)
	return &c, nil
}

func (m *communityModel) ListComments(ctx context.Context, postNo, status string, page, size int) (int64, []types.Comment, error) {
	_, size, offset := pageArgs(page, size)
	where := " WHERE 1=1"
	args := []interface{}{}
	if postNo != "" {
		where += " AND post_no=?"
		args = append(args, postNo)
	}
	if status != "" {
		where += " AND status=?"
		args = append(args, status)
	}
	var total int64
	if err := m.conn.QueryRowCtx(ctx, &total, "SELECT COUNT(*) FROM post_comment"+where, args...); err != nil {
		return 0, nil, err
	}
	var rows []commentRow
	qargs := append(append([]interface{}{}, args...), size, offset)
	if err := m.conn.QueryRowsCtx(ctx, &rows, "SELECT comment_no,post_no,user_id,content,status,audit_id,audit_remark,create_time FROM post_comment"+where+" ORDER BY create_time DESC LIMIT ? OFFSET ?", qargs...); err != nil {
		return 0, nil, err
	}
	list := make([]types.Comment, 0, len(rows))
	for i := range rows {
		list = append(list, rowToComment(&rows[i]))
	}
	return total, list, nil
}
func (m *communityModel) ReviewComment(ctx context.Context, id, auditor, status, remark string) (*types.Comment, error) {
	err := m.conn.TransactCtx(ctx, func(_ context.Context, s sqlx.Session) error {
		return review(ctx, s, "post_comment", "comment_no", id, auditor, status, remark, true)
	})
	if err != nil {
		return nil, err
	}
	var row commentRow
	if err = m.conn.QueryRowCtx(ctx, &row, "SELECT comment_no,post_no,user_id,content,status,audit_id,audit_remark,create_time FROM post_comment WHERE comment_no=?", id); err != nil {
		return nil, err
	}
	c := rowToComment(&row)
	return &c, nil
}
func (m *communityModel) SetFollow(ctx context.Context, masterId, userId string, following bool) (bool, error) {
	if following {
		_, err := m.conn.ExecCtx(ctx, "INSERT IGNORE INTO master_follow(master_id,user_id) VALUES(?,?)", masterId, userId)
		return true, err
	}
	_, err := m.conn.ExecCtx(ctx, "DELETE FROM master_follow WHERE master_id=? AND user_id=?", masterId, userId)
	return false, err
}

func rowToPost(r *postRow) types.Post {
	return types.Post{Id: r.PostNo, MasterId: r.MasterId, OwnerId: r.OwnerId, Type: r.Type, Title: r.Title, Content: r.Content, CoverMediaId: r.CoverMediaId, BeliefCode: r.BeliefCode, Status: r.Status, AuditRemark: r.AuditRemark, LikeCount: r.LikeCount, CommentCount: r.CommentCount, CreateTime: r.CreateTime, UpdateTime: r.UpdateTime, Assets: []types.Asset{}}
}
func rowToComment(r *commentRow) types.Comment {
	return types.Comment{Id: r.CommentNo, PostId: r.PostNo, UserId: r.UserId, Content: r.Content, Status: r.Status, AuditRemark: r.AuditRemark, CreateTime: r.CreateTime}
}

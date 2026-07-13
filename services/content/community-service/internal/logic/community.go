package logic

import (
	"context"
	"strings"

	"github.com/askxuan/common"
	"github.com/askxuan/community-service/internal/svc"
	"github.com/askxuan/community-service/internal/types"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func Feed(ctx context.Context, s *svc.ServiceContext, req *types.FeedReq) (*types.PostListResp, error) {
	page, size := normalizePage(req.Page, req.Size)
	total, list, err := s.Model.ListPosts(ctx, "approved", req.Type, req.BeliefCode, "", page, size)
	if err != nil {
		return nil, common.ErrSystem
	}
	for i := range list {
		list[i].OwnerId = ""
		list[i].AuditRemark = ""
	}
	return &types.PostListResp{Total: total, List: list, Page: page, Size: size}, nil
}
func PostDetail(ctx context.Context, s *svc.ServiceContext, id, viewer string) (*types.Post, error) {
	p, err := s.Model.FindPost(ctx, id, viewer, false)
	if err != nil {
		return nil, mapError(err)
	}
	p.OwnerId = ""
	p.AuditRemark = ""
	return p, nil
}
func Comments(ctx context.Context, s *svc.ServiceContext, req *types.CommentListReq) (*types.CommentListResp, error) {
	page, size := normalizePage(req.Page, req.Size)
	total, list, err := s.Model.ListComments(ctx, req.Id, "approved", page, size)
	if err != nil {
		return nil, common.ErrSystem
	}
	return &types.CommentListResp{Total: total, List: list, Page: page, Size: size}, nil
}
func CreateComment(ctx context.Context, s *svc.ServiceContext, req *types.CommentCreateReq) (*types.Comment, error) {
	req.Content = strings.TrimSpace(req.Content)
	if req.Id == "" || req.UserId == "" || len([]rune(req.Content)) < 1 || len([]rune(req.Content)) > 500 {
		return nil, common.ErrParam
	}
	c, err := s.Model.CreateComment(ctx, req.Id, req.UserId, req.Content)
	if err != nil {
		return nil, mapError(err)
	}
	return c, nil
}
func SetLike(ctx context.Context, s *svc.ServiceContext, req *types.LikeReq, liked bool) (*types.LikeResp, error) {
	if req.Id == "" || req.UserId == "" {
		return nil, common.ErrParam
	}
	r, err := s.Model.SetLike(ctx, req.Id, req.UserId, liked)
	if err != nil {
		return nil, mapError(err)
	}
	return r, nil
}
func SetFollow(ctx context.Context, s *svc.ServiceContext, req *types.FollowReq, following bool) (*types.FollowResp, error) {
	if req.Id == "" || req.UserId == "" {
		return nil, common.ErrParam
	}
	value, err := s.Model.SetFollow(ctx, req.Id, req.UserId, following)
	if err != nil {
		return nil, common.ErrSystem
	}
	return &types.FollowResp{Following: value}, nil
}

func MasterPosts(ctx context.Context, s *svc.ServiceContext, req *types.MasterPostListReq) (*types.PostListResp, error) {
	page, size := normalizePage(req.Page, req.Size)
	total, list, err := s.Model.ListPosts(ctx, req.Status, "", "", req.OwnerId, page, size)
	if err != nil {
		return nil, common.ErrSystem
	}
	return &types.PostListResp{Total: total, List: list, Page: page, Size: size}, nil
}
func WritePost(ctx context.Context, s *svc.ServiceContext, req *types.PostWriteReq, update bool) (*types.Post, error) {
	if err := validatePost(req); err != nil {
		return nil, err
	}
	if err := s.Model.ValidateAssets(ctx, req.OwnerId, req.CoverMediaId, req.Assets); err != nil {
		return nil, common.NewBizError(40941, "媒体资产不存在、未完成或不属于当前法师")
	}
	var p *types.Post
	var err error
	if update {
		p, err = s.Model.UpdatePost(ctx, req)
	} else {
		p, err = s.Model.CreatePost(ctx, req)
	}
	if err != nil {
		return nil, mapError(err)
	}
	return p, nil
}
func ChangePostStatus(ctx context.Context, s *svc.ServiceContext, req *types.PostStatusReq) (*types.Post, error) {
	if req.Status != "submit" && req.Status != "off_shelf" {
		return nil, common.ErrParam
	}
	p, err := s.Model.ChangePostStatus(ctx, req.Id, req.OwnerId, req.Status)
	if err != nil {
		return nil, mapError(err)
	}
	return p, nil
}

func AdminPosts(ctx context.Context, s *svc.ServiceContext, req *types.AdminListReq) (*types.PostListResp, error) {
	page, size := normalizePage(req.Page, req.Size)
	total, list, err := s.Model.ListPosts(ctx, req.Status, "", "", "", page, size)
	if err != nil {
		return nil, common.ErrSystem
	}
	return &types.PostListResp{Total: total, List: list, Page: page, Size: size}, nil
}
func ReviewPost(ctx context.Context, s *svc.ServiceContext, req *types.AdminReviewReq, status string) (*types.Post, error) {
	if req.Id == "" || req.AuditorId == "" || (status == "rejected" && strings.TrimSpace(req.Remark) == "") {
		return nil, common.ErrParam
	}
	p, err := s.Model.ReviewPost(ctx, req.Id, req.AuditorId, status, req.Remark)
	if err != nil {
		logx.WithContext(ctx).Errorf("review community post %s failed: %v", req.Id, err)
		return nil, mapError(err)
	}
	return p, nil
}
func AdminComments(ctx context.Context, s *svc.ServiceContext, req *types.AdminCommentListReq) (*types.CommentListResp, error) {
	page, size := normalizePage(req.Page, req.Size)
	total, list, err := s.Model.ListComments(ctx, "", req.Status, page, size)
	if err != nil {
		return nil, common.ErrSystem
	}
	return &types.CommentListResp{Total: total, List: list, Page: page, Size: size}, nil
}
func ReviewComment(ctx context.Context, s *svc.ServiceContext, req *types.AdminReviewReq, status string) (*types.Comment, error) {
	if req.Id == "" || req.AuditorId == "" || (status == "rejected" && strings.TrimSpace(req.Remark) == "") {
		return nil, common.ErrParam
	}
	c, err := s.Model.ReviewComment(ctx, req.Id, req.AuditorId, status, req.Remark)
	if err != nil {
		return nil, mapError(err)
	}
	return c, nil
}

func validatePost(req *types.PostWriteReq) error {
	req.Title = strings.TrimSpace(req.Title)
	req.Content = strings.TrimSpace(req.Content)
	if req.OwnerId == "" || req.MasterId == "" || len([]rune(req.Title)) < 1 || len([]rune(req.Title)) > 120 {
		return common.ErrParam
	}
	if req.Type != "article" && req.Type != "video" {
		return common.ErrParam
	}
	images, videos := 0, 0
	seen := map[int64]bool{}
	for _, a := range req.Assets {
		if a.MediaId <= 0 || seen[a.MediaId] || (a.AssetType != "image" && a.AssetType != "video") {
			return common.ErrParam
		}
		seen[a.MediaId] = true
		if a.AssetType == "image" {
			images++
		} else {
			videos++
		}
	}
	if req.Type == "video" && (videos != 1 || images > 1) {
		return common.NewBizError(40031, "视频帖必须包含一个视频，可选一张封面")
	}
	if req.Type == "article" && (images < 1 || images > 9 || videos > 0) {
		return common.NewBizError(40032, "图文帖需要 1 至 9 张图片")
	}
	return nil
}
func normalizePage(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 50 {
		size = 20
	}
	return page, size
}
func mapError(err error) error {
	if err == sqlx.ErrNotFound {
		return common.NewBizError(40440, "社区内容不存在或状态已变化")
	}
	return common.ErrSystem
}

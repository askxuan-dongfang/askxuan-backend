package handler

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/community-service/internal/logic"
	"github.com/askxuan/community-service/internal/svc"
	"github.com/askxuan/community-service/internal/types"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func RegisterHandlers(server *rest.Server, s *svc.ServiceContext) {
	server.Use(middleware.CorsFunc)
	server.AddRoutes([]rest.Route{
		{Method: http.MethodGet, Path: "/api/v1/community/feed", Handler: feed(s)}, {Method: http.MethodGet, Path: "/api/v1/community/posts/:id", Handler: postDetail(s)},
		{Method: http.MethodPost, Path: "/api/v1/community/posts/:id/like", Handler: like(s, true)}, {Method: http.MethodDelete, Path: "/api/v1/community/posts/:id/like", Handler: like(s, false)},
		{Method: http.MethodGet, Path: "/api/v1/community/posts/:id/comments", Handler: comments(s)}, {Method: http.MethodPost, Path: "/api/v1/community/posts/:id/comments", Handler: createComment(s)},
		{Method: http.MethodPost, Path: "/api/v1/community/masters/:id/follow", Handler: follow(s, true)}, {Method: http.MethodDelete, Path: "/api/v1/community/masters/:id/follow", Handler: follow(s, false)},
		{Method: http.MethodGet, Path: "/api/v1/community/masters/following", Handler: myFollowing(s)},
		{Method: http.MethodGet, Path: "/api/v1/admin/masters/community/posts", Handler: masterPosts(s)}, {Method: http.MethodPost, Path: "/api/v1/admin/masters/community/posts", Handler: createPost(s)},
		{Method: http.MethodPut, Path: "/api/v1/admin/masters/community/posts/:id", Handler: updatePost(s)}, {Method: http.MethodPut, Path: "/api/v1/admin/masters/community/posts/:id/status", Handler: postStatus(s)},
		{Method: http.MethodGet, Path: "/api/v1/admin/platform/community/posts", Handler: adminPosts(s)}, {Method: http.MethodPut, Path: "/api/v1/admin/platform/community/posts/:id/approve", Handler: reviewPost(s, "approved")}, {Method: http.MethodPut, Path: "/api/v1/admin/platform/community/posts/:id/reject", Handler: reviewPost(s, "rejected")},
		{Method: http.MethodGet, Path: "/api/v1/admin/platform/community/comments", Handler: adminComments(s)}, {Method: http.MethodPut, Path: "/api/v1/admin/platform/community/comments/:id/approve", Handler: reviewComment(s, "approved")}, {Method: http.MethodPut, Path: "/api/v1/admin/platform/community/comments/:id/reject", Handler: reviewComment(s, "rejected")},
	})
}
func myFollowing(s *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v, e := logic.MyFollowing(r.Context(), s, r.Header.Get("X-User-Id"))
		respond(w, v, e)
	}
}
func feed(s *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var q types.FeedReq
		if !parse(w, r, &q) {
			return
		}
		v, e := logic.Feed(r.Context(), s, &q)
		respond(w, v, e)
	}
}
func postDetail(s *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var q types.PostDetailReq
		if !parse(w, r, &q) {
			return
		}
		v, e := logic.PostDetail(r.Context(), s, q.Id, r.Header.Get("X-User-Id"))
		respond(w, v, e)
	}
}
func comments(s *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var q types.CommentListReq
		if !parse(w, r, &q) {
			return
		}
		v, e := logic.Comments(r.Context(), s, &q)
		respond(w, v, e)
	}
}
func createComment(s *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var q types.CommentCreateReq
		if !parse(w, r, &q) {
			return
		}
		var e error
		q.UserId, e = owner(r, q.UserId)
		if e != nil {
			respond(w, nil, e)
			return
		}
		v, e := logic.CreateComment(r.Context(), s, &q)
		respond(w, v, e)
	}
}
func like(s *svc.ServiceContext, value bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var q types.LikeReq
		if !parse(w, r, &q) {
			return
		}
		var e error
		q.UserId, e = owner(r, q.UserId)
		if e != nil {
			respond(w, nil, e)
			return
		}
		v, e := logic.SetLike(r.Context(), s, &q, value)
		respond(w, v, e)
	}
}
func follow(s *svc.ServiceContext, value bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var q types.FollowReq
		if !parse(w, r, &q) {
			return
		}
		var e error
		q.UserId, e = owner(r, q.UserId)
		if e != nil {
			respond(w, nil, e)
			return
		}
		v, e := logic.SetFollow(r.Context(), s, &q, value)
		respond(w, v, e)
	}
}
func masterIdentity(r *http.Request, ownerReq, masterReq string) (string, string, error) {
	o, e := owner(r, ownerReq)
	if e != nil {
		return "", "", e
	}
	m := r.Header.Get("X-Master-Id")
	if r.Header.Get("X-User-Id") != "" {
		if m == "" || (masterReq != "" && masterReq != m) {
			return "", "", common.ErrRoleForbidden
		}
		return o, m, nil
	}
	if masterReq == "" {
		return "", "", common.ErrRoleForbidden
	}
	return o, masterReq, nil
}
func masterPosts(s *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var q types.MasterPostListReq
		if !parse(w, r, &q) {
			return
		}
		o, m, e := masterIdentity(r, q.OwnerId, q.MasterId)
		if e != nil {
			respond(w, nil, e)
			return
		}
		q.OwnerId, q.MasterId = o, m
		v, e := logic.MasterPosts(r.Context(), s, &q)
		respond(w, v, e)
	}
}
func createPost(s *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var q types.PostWriteReq
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			respond(w, nil, common.ErrParam)
			return
		}
		o, m, e := masterIdentity(r, q.OwnerId, q.MasterId)
		if e != nil {
			respond(w, nil, e)
			return
		}
		q.OwnerId, q.MasterId = o, m
		v, e := logic.WritePost(r.Context(), s, &q, false)
		respond(w, v, e)
	}
}
func updatePost(s *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var q types.PostWriteReq
		var path types.PostDetailReq
		if err := httpx.ParsePath(r, &path); err != nil {
			respond(w, nil, common.ErrParam)
			return
		}
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			respond(w, nil, common.ErrParam)
			return
		}
		q.Id = path.Id
		o, m, e := masterIdentity(r, q.OwnerId, q.MasterId)
		if e != nil {
			respond(w, nil, e)
			return
		}
		q.OwnerId, q.MasterId = o, m
		v, e := logic.WritePost(r.Context(), s, &q, true)
		respond(w, v, e)
	}
}
func postStatus(s *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var q types.PostStatusReq
		if !parse(w, r, &q) {
			return
		}
		o, e := owner(r, q.OwnerId)
		if e != nil {
			respond(w, nil, e)
			return
		}
		q.OwnerId = o
		v, e := logic.ChangePostStatus(r.Context(), s, &q)
		respond(w, v, e)
	}
}
func adminPosts(s *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if e := requirePlatformRole(r); e != nil {
			respond(w, nil, e)
			return
		}
		var q types.AdminListReq
		if !parse(w, r, &q) {
			return
		}
		v, e := logic.AdminPosts(r.Context(), s, &q)
		respond(w, v, e)
	}
}
func adminComments(s *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if e := requirePlatformRole(r); e != nil {
			respond(w, nil, e)
			return
		}
		var q types.AdminCommentListReq
		if !parse(w, r, &q) {
			return
		}
		v, e := logic.AdminComments(r.Context(), s, &q)
		respond(w, v, e)
	}
}
func reviewPost(s *svc.ServiceContext, status string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if e := requirePlatformRole(r); e != nil {
			respond(w, nil, e)
			return
		}
		var q types.AdminReviewReq
		if !parse(w, r, &q) {
			return
		}
		if id := r.Header.Get("X-User-Id"); id != "" {
			q.AuditorId = id
		} else if id := r.Header.Get("X-Auditor-Id"); id != "" {
			q.AuditorId = id
		}
		v, e := logic.ReviewPost(r.Context(), s, &q, status)
		respond(w, v, e)
	}
}
func reviewComment(s *svc.ServiceContext, status string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if e := requirePlatformRole(r); e != nil {
			respond(w, nil, e)
			return
		}
		var q types.AdminReviewReq
		if !parse(w, r, &q) {
			return
		}
		if id := r.Header.Get("X-User-Id"); id != "" {
			q.AuditorId = id
		} else if id := r.Header.Get("X-Auditor-Id"); id != "" {
			q.AuditorId = id
		}
		v, e := logic.ReviewComment(r.Context(), s, &q, status)
		respond(w, v, e)
	}
}
func requirePlatformRole(r *http.Request) error {
	if r.Header.Get("X-User-Id") == "" {
		return nil
	}
	for _, role := range strings.Split(r.Header.Get("X-User-Roles"), ",") {
		if role == "platform_super" || role == "platform_service" {
			return nil
		}
	}
	return common.ErrRoleForbidden
}
func owner(r *http.Request, requested string) (string, error) {
	trusted := r.Header.Get("X-User-Id")
	if trusted != "" {
		if requested != "" && !constantEqual(requested, trusted) {
			return "", common.ErrForbidden
		}
		return trusted, nil
	}
	if requested == "" {
		return "", common.ErrForbidden
	}
	return requested, nil
}
func constantEqual(a, b string) bool {
	return len(a) == len(b) && subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
func parse(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	if e := httpx.Parse(r, v); e != nil {
		common.JsonError(w, common.ErrParam)
		return false
	}
	return true
}
func respond(w http.ResponseWriter, v interface{}, e error) {
	if e != nil {
		common.JsonError(w, e)
		return
	}
	common.Ok(w, v)
}

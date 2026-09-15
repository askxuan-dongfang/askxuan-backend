package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/askxuan/auth-service/internal/svc"
	"github.com/askxuan/common"
	"github.com/askxuan/common/identity"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/rest"
)

type workActor struct {
	ID           int64
	Role, Temple string
}

func authenticateWork(r *http.Request, sc *svc.ServiceContext) (workActor, error) {
	empty := workActor{}
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	c, e := common.ParseToken(sc.Config.Auth.AccessSecret, token)
	if e != nil || c.IsRefreshToken() || c.UserType != "admin" || sc.SessionRedis == nil {
		return empty, common.ErrUnauthorized
	}
	if e = identity.CheckSession(r.Context(), sc.SessionRedis, c.SessionID, "admin", c.UserId); e != nil {
		return empty, common.ErrTokenInvalid
	}
	a, e := sc.AdminAccountModel.FindByID(r.Context(), c.UserId)
	if e != nil || a.Status != "enabled" {
		return empty, common.ErrForbidden
	}
	role, e := sc.RoleModel.FindByID(r.Context(), a.RoleId)
	if e != nil || !c.HasRole(role.Code) {
		return empty, common.ErrForbidden
	}
	if role.Code == "temple_admin" {
		var n int64
		if e = sc.DB.QueryRowCtx(r.Context(), &n, "SELECT COUNT(*) FROM askxuan_temple.temple t JOIN askxuan_temple.temple_admin a ON a.temple_code=t.code WHERE t.code=? AND a.account_id=? AND t.status IN ('正常','推荐')", a.TempleId, a.Id); e != nil || n != 1 {
			return empty, common.ErrForbidden
		}
	}
	return workActor{ID: a.Id, Role: role.Code, Temple: a.TempleId}, nil
}
func onboardingError(w http.ResponseWriter, e error) {
	// Never return database errors (which may contain private metadata).
	switch e {
	case common.ErrUnauthorized, common.ErrTokenInvalid, common.ErrForbidden:
		common.JsonError(w, e)
		return
	}
	msg := e.Error()
	if strings.Contains(msg, "Error ") || strings.Contains(msg, "sql:") || e == sqlx.ErrNotFound {
		msg = "资料不存在或操作未完成，请刷新后重试"
	}
	identityError(w, fmt.Errorf("%s", msg))
}
func readWorkJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 20000)
	if json.NewDecoder(r.Body).Decode(v) != nil {
		common.JsonError(w, common.ErrParam)
		return false
	}
	return true
}
func registerOnboardingHandlers(server *rest.Server, sc *svc.ServiceContext) {
	add := func(method, path string, f func(http.ResponseWriter, *http.Request, workActor)) {
		options := []rest.RouteOption{}
		if method == "POST" && path == "/evidence" {
			options = append(options, rest.WithMaxBytes(5*1024*1024+4096))
		}
		server.AddRoute(rest.Route{Method: method, Path: "/api/v1/auth/onboarding" + path, Handler: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			actor, e := authenticateWork(r, sc)
			if e != nil {
				onboardingError(w, e)
				return
			}
			f(w, r, actor)
		}}, options...)
	}
	add("GET", "/application", func(w http.ResponseWriter, r *http.Request, a workActor) {
		app, e := sc.Accounts.WorkApplication(r.Context(), a.ID)
		if e != nil {
			onboardingError(w, e)
			return
		}
		common.Ok(w, app)
	})
	add("POST", "/application", func(w http.ResponseWriter, r *http.Request, a workActor) {
		if !identity.IsApplicant(a.Role) {
			common.JsonError(w, common.ErrForbidden)
			return
		}
		var q struct {
			Profile  identity.WorkProfile `json:"profile"`
			Revision int                  `json:"revision"`
			Submit   bool                 `json:"submit"`
		}
		if !readWorkJSON(w, r, &q) {
			return
		}
		app, e := sc.Accounts.SaveWorkApplication(r.Context(), a.ID, q.Profile, q.Revision, q.Submit)
		if e != nil {
			onboardingError(w, e)
			return
		}
		common.Ok(w, app)
	})
	add("GET", "/reviews", func(w http.ResponseWriter, r *http.Request, a workActor) {
		if a.Role != "platform_super" {
			common.JsonError(w, common.ErrForbidden)
			return
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		rows, e := sc.Accounts.WorkApplications(r.Context(), r.URL.Query().Get("status"), page)
		if e != nil {
			onboardingError(w, e)
			return
		}
		common.Ok(w, map[string]any{"list": rows})
	})
	add("POST", "/review", func(w http.ResponseWriter, r *http.Request, a workActor) {
		if a.Role != "platform_super" {
			common.JsonError(w, common.ErrForbidden)
			return
		}
		var q struct {
			ID       int64  `json:"id"`
			Revision int    `json:"revision"`
			Approve  bool   `json:"approve"`
			Note     string `json:"note"`
		}
		if !readWorkJSON(w, r, &q) {
			return
		}
		if e := sc.Accounts.ReviewWorkApplication(r.Context(), a.ID, q.ID, q.Revision, q.Approve, q.Note); e != nil {
			onboardingError(w, e)
			return
		}
		common.Ok(w, map[string]bool{"success": true})
	})
	add("GET", "/history", func(w http.ResponseWriter, r *http.Request, a workActor) {
		id, _ := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		var owner int64
		if e := sc.DB.QueryRowCtx(r.Context(), &owner, "SELECT account_id FROM onboarding_application WHERE id=?", id); e != nil || (a.Role != "platform_super" && owner != a.ID) {
			common.JsonError(w, common.ErrForbidden)
			return
		}
		type event struct {
			Action   string `db:"action" json:"action"`
			Revision int    `db:"revision" json:"revision"`
			Note     string `db:"note" json:"note"`
			At       string `db:"at" json:"at"`
		}
		rows := []event{}
		if e := sc.DB.QueryRowsCtx(r.Context(), &rows, "SELECT action,revision,note,DATE_FORMAT(created_at,'%Y-%m-%d %H:%i:%s') at FROM onboarding_event WHERE application_id=? ORDER BY id DESC LIMIT 100", id); e != nil {
			onboardingError(w, e)
			return
		}
		common.Ok(w, map[string]any{"list": rows})
	})
	add("POST", "/evidence", func(w http.ResponseWriter, r *http.Request, a workActor) {
		if !identity.IsApplicant(a.Role) {
			common.JsonError(w, common.ErrForbidden)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 5*1024*1024+4096)
		if e := r.ParseMultipartForm(5 * 1024 * 1024); e != nil {
			onboardingError(w, fmt.Errorf("每份材料不能超过 5MB"))
			return
		}
		defer r.MultipartForm.RemoveAll()
		f, h, e := r.FormFile("file")
		if e != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		defer f.Close()
		b, e := io.ReadAll(io.LimitReader(f, 5*1024*1024+1))
		if e != nil || len(b) == 0 || len(b) > 5*1024*1024 {
			common.JsonError(w, common.ErrParam)
			return
		}
		typ := http.DetectContentType(b)
		if typ != "application/pdf" && typ != "image/png" && typ != "image/jpeg" {
			onboardingError(w, fmt.Errorf("仅支持 PDF、PNG、JPEG"))
			return
		}
		name := filepath.Base(h.Filename)
		if len(name) > 180 {
			name = "credential"
		}
		id := identity.RandomID()
		e = sc.DB.TransactCtx(r.Context(), func(ctx context.Context, tx sqlx.Session) error {
			var owner int64
			if e := tx.QueryRowCtx(ctx, &owner, "SELECT account_id FROM onboarding_application WHERE account_id=? AND status IN ('draft','rejected') FOR UPDATE", a.ID); e != nil {
				return e
			}
			var count int64
			if e := tx.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM onboarding_evidence WHERE account_id=?", a.ID); e != nil {
				return e
			}
			if count >= 24 {
				return fmt.Errorf("材料存储已达上限，请联系平台处理")
			}
			_, e := tx.ExecCtx(ctx, "INSERT INTO onboarding_evidence(id,account_id,filename,media_type,content) VALUES (?,?,?,?,?)", id, a.ID, name, typ, b)
			return e
		})
		if e != nil {
			onboardingError(w, e)
			return
		}
		common.Ok(w, map[string]string{"id": id, "name": name})
	})
	add("GET", "/evidence", func(w http.ResponseWriter, r *http.Request, a workActor) {
		var row struct {
			Owner   int64  `db:"account_id"`
			Name    string `db:"filename"`
			Type    string `db:"media_type"`
			Content []byte `db:"content"`
		}
		if e := sc.DB.QueryRowCtx(r.Context(), &row, "SELECT account_id,filename,media_type,content FROM onboarding_evidence WHERE id=?", r.URL.Query().Get("id")); e != nil || (row.Owner != a.ID && a.Role != "platform_super") {
			common.JsonError(w, common.ErrForbidden)
			return
		}
		w.Header().Set("Content-Type", row.Type)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": row.Name}))
		_, _ = w.Write(row.Content)
	})
	add("GET", "/managed/candidates", func(w http.ResponseWriter, r *http.Request, a workActor) {
		if a.Role != "temple_admin" {
			common.JsonError(w, common.ErrForbidden)
			return
		}
		type candidate struct {
			Code string `db:"code" json:"code"`
			Name string `db:"dharma_name" json:"name"`
		}
		rows := []candidate{}
		e := sc.DB.QueryRowsCtx(r.Context(), &rows, "SELECT m.code,m.dharma_name FROM askxuan_master.master m WHERE m.temple_code=? AND m.manage_by='temple' AND m.auth_status='已认证' AND m.platform_status='normal' AND NOT EXISTS (SELECT 1 FROM admin_account a WHERE a.master_id=m.code) ORDER BY m.id DESC LIMIT 200", a.Temple)
		if e != nil {
			onboardingError(w, e)
			return
		}
		common.Ok(w, map[string]any{"list": rows})
	})
	add("GET", "/managed", func(w http.ResponseWriter, r *http.Request, a workActor) {
		if a.Role != "temple_admin" {
			common.JsonError(w, common.ErrForbidden)
			return
		}
		rows, e := sc.Accounts.ManagedInvitations(r.Context(), a.Temple)
		if e != nil {
			onboardingError(w, e)
			return
		}
		common.Ok(w, map[string]any{"list": rows})
	})
	add("POST", "/managed", func(w http.ResponseWriter, r *http.Request, a workActor) {
		if a.Role != "temple_admin" {
			common.JsonError(w, common.ErrForbidden)
			return
		}
		var q struct {
			Master   string `json:"masterCode"`
			Email    string `json:"email"`
			Username string `json:"username"`
		}
		if !readWorkJSON(w, r, &q) {
			return
		}
		if e := sc.Accounts.InviteManagedMaster(r.Context(), a.ID, a.Temple, q.Master, q.Email, q.Username); e != nil {
			onboardingError(w, e)
			return
		}
		common.Ok(w, map[string]bool{"success": true})
	})
	add("POST", "/managed/revoke", func(w http.ResponseWriter, r *http.Request, a workActor) {
		if a.Role != "temple_admin" {
			common.JsonError(w, common.ErrForbidden)
			return
		}
		var q struct {
			ID int64 `json:"id"`
		}
		if !readWorkJSON(w, r, &q) {
			return
		}
		if e := sc.Accounts.RevokeManagedMaster(r.Context(), a.Temple, q.ID); e != nil {
			onboardingError(w, e)
			return
		}
		common.Ok(w, map[string]bool{"success": true})
	})
	add("POST", "/managed/restore", func(w http.ResponseWriter, r *http.Request, a workActor) {
		if a.Role != "temple_admin" {
			common.JsonError(w, common.ErrForbidden)
			return
		}
		var q struct {
			ID int64 `json:"id"`
		}
		if !readWorkJSON(w, r, &q) {
			return
		}
		if e := sc.Accounts.RestoreManagedMaster(r.Context(), a.Temple, q.ID); e != nil {
			onboardingError(w, e)
			return
		}
		common.Ok(w, map[string]bool{"success": true})
	})
}

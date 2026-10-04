package weknora

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"strconv"
	"strings"
)

type WikiEdit struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Summary string `json:"summary"`
	Version int    `json:"version"`
	Status  string `json:"status"`
	Issue   string `json:"issue"`
}
type WikiRevision struct {
	Version int    `json:"version"`
	Title   string `json:"title"`
	Content string `json:"content,omitempty"`
	Summary string `json:"summary"`
	Edited  string `json:"edited_at"`
	Source  string `json:"edit_source"`
}
type WikiHistory struct {
	Revisions []WikiRevision `json:"revisions"`
	Total     int            `json:"total"`
	Current   int            `json:"current_version"`
}
type WikiIssue struct {
	ID          string `json:"id"`
	Slug        string `json:"slug"`
	Type        string `json:"issue_type"`
	Status      string `json:"status"`
	Description string `json:"description"`
}
type WikiLint struct {
	Health  int    `json:"health_score"`
	Summary string `json:"summary"`
	Issues  []struct {
		Type        string `json:"type"`
		Severity    string `json:"severity"`
		Slug        string `json:"page_slug"`
		Target      string `json:"target_slug"`
		Description string `json:"description"`
		Fixable     bool   `json:"auto_fixable"`
	} `json:"issues"`
}

// The portal exposes a closed editorial API, never arbitrary engine paths.
func (s *Service) WikiWrite(ctx context.Context, kb, kind, actor string, in WikiEdit) (any, error) {
	if !validID(kb) {
		return nil, ErrInput
	}
	var out any
	err := s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var b Base
		if e := tx.QueryRowCtx(ctx, &b, "SELECT id,name,description,enabled,revision FROM ai_knowledge_base WHERE id=? FOR UPDATE", kb); e != nil {
			return e
		}
		state, e := s.WikiStatus(ctx, kb)
		if e != nil {
			return e
		}
		if !state.Indexing.Wiki {
			return ErrInput
		}
		method, suffix := "POST", kind
		var payload any = map[string]any{}
		switch kind {
		case "create", "edit", "delete", "revert":
			slug, e := wikiSlug(in.Slug)
			if e != nil {
				return e
			}
			if kind == "edit" || kind == "delete" || kind == "revert" {
				p, e := s.WikiRead(ctx, kb, "page", in.Slug, "", 1)
				if e != nil {
					return e
				}
				if in.Version < 1 || p.(*WikiPage).Version != in.Version {
					return ErrConflict
				}
			}
			switch kind {
			case "create", "edit":
				if strings.TrimSpace(in.Title) == "" || len([]rune(in.Title)) > 200 || len(in.Content) > 180000 || len(in.Summary) > 4000 {
					return ErrInput
				}
				if in.Status != "draft" && in.Status != "published" && in.Status != "archived" {
					return ErrInput
				}
				payload = map[string]any{"title": in.Title, "content": in.Content, "summary": in.Summary, "status": in.Status, "version": in.Version}
				suffix = "pages/" + slug
				if kind == "create" {
					suffix = "pages"
					payload.(map[string]any)["slug"] = in.Slug
					payload.(map[string]any)["page_type"] = "concept"
				} else {
					method = "PUT"
				}
			case "delete":
				method = "DELETE"
				suffix = "pages/" + slug
				payload = nil
			case "revert":
				// Revision requested in Issue is kept separate from optimistic current Version.
				target, e := strconv.Atoi(in.Issue)
				if e != nil || target < 1 || target >= in.Version {
					return ErrInput
				}
				// Restore via a version-checked edit; upstream /revert lacks an expected-version guard.
				raw, e := s.Client.raw(ctx, "GET", "/knowledgebase/"+kb+"/wiki/revisions/"+slug+"?version="+strconv.Itoa(target), nil, "")
				if e != nil {
					return e
				}
				var rev WikiRevision
				if json.Unmarshal(raw, &rev) != nil || rev.Version != target {
					return ErrUnavailable
				}
				method = "PUT"
				suffix = "pages/" + slug
				payload = map[string]any{"title": rev.Title, "content": rev.Content, "summary": rev.Summary, "version": in.Version}
			}
		case "issue":
			if !validID(in.Issue) || (in.Status != "pending" && in.Status != "resolved" && in.Status != "ignored") {
				return ErrInput
			}
			method = "PUT"
			suffix = "issues/" + in.Issue + "/status"
			payload = map[string]string{"status": in.Status}
		case "rebuild-links", "auto-fix":
		default:
			return ErrInput
		}
		encoded, _ := json.Marshal(payload)
		raw, e := s.Client.raw(ctx, method, "/knowledgebase/"+kb+"/wiki/"+suffix, bytes.NewReader(encoded), "application/json")
		if e != nil {
			return e
		}
		if kind == "create" || kind == "edit" || kind == "revert" {
			var p WikiPage
			if json.Unmarshal(raw, &p) != nil {
				return ErrUnavailable
			}
			out = p
		} else {
			out = map[string]bool{"ok": true}
		}
		return s.audit(ctx, tx, actor, "wiki_"+kind, kb)
	})
	return out, err
}

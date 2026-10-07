package weknora

import (
	"context"
	"encoding/json"
	"net/url"
	"strconv"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// Explicit DTOs prevent engine model credentials and storage configuration from
// reaching the portal. Wiki is an editorial view, never an authorization bypass
// into the Harness document retrieval scope.
type WikiState struct {
	SummaryModel   string `json:"summary_model_id"`
	EmbeddingModel string `json:"embedding_model_id"`
	Revision       int64  `json:"revision"`
	Documents      int    `json:"knowledge_count"`
	Chunks         int    `json:"chunk_count"`
	Processing     int    `json:"processing_count"`
	Indexing       struct {
		Wiki  bool `json:"wiki_enabled"`
		Graph bool `json:"graph_enabled"`
	} `json:"indexing_strategy"`
	Config *WikiConfig `json:"wiki_config"`
}
type WikiConfig struct {
	Model                  string `json:"synthesis_model_id"`
	Granularity            string `json:"extraction_granularity"`
	MaxPages               int    `json:"max_pages_per_ingest"`
	ContentInstructions    string `json:"content_instructions"`
	ExtractionInstructions string `json:"extraction_instructions"`
}

type WikiReference struct {
	ID       string `json:"id"`
	Source   string `json:"source"`
	Approved bool   `json:"approved"`
}
type WikiPage struct {
	References []WikiReference `json:"references"`
	Slug       string          `json:"slug"`
	Title      string          `json:"title"`
	Type       string          `json:"page_type"`
	Status     string          `json:"status"`
	Content    string          `json:"content"`
	Summary    string          `json:"summary"`
	Version    int             `json:"version"`
	Sources    []string        `json:"source_refs"`
	In         []string        `json:"in_links"`
	Out        []string        `json:"out_links"`
	Updated    string          `json:"updated_at"`
}
type WikiStats struct {
	Total   int            `json:"total_pages"`
	Types   map[string]int `json:"pages_by_type"`
	Links   int            `json:"total_links"`
	Orphans int            `json:"orphan_count"`
	Pending int            `json:"pending_tasks"`
	Issues  int            `json:"pending_issues"`
	Active  bool           `json:"is_active"`
}
type WikiGraph struct {
	Nodes []struct {
		Slug  string `json:"slug"`
		Title string `json:"title"`
		Type  string `json:"page_type"`
	} `json:"nodes"`
	Edges []struct {
		Source string `json:"source"`
		Target string `json:"target"`
	} `json:"edges"`
	Meta struct {
		Total     int  `json:"total"`
		Returned  int  `json:"returned"`
		Truncated bool `json:"truncated"`
	} `json:"meta"`
}

func (s *Service) WikiStatus(ctx context.Context, kb string) (WikiState, error) {
	base, e := s.base(ctx, kb)
	if e != nil {
		return WikiState{}, e
	}
	state, e := decode[WikiState](s.Client.json(ctx, "GET", "/knowledge-bases/"+kb, nil))
	state.Revision = base.Revision
	return state, e
}

type WikiModel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

func (s *Service) WikiModels(ctx context.Context, kb string) ([]WikiModel, error) {
	if _, e := s.base(ctx, kb); e != nil {
		return nil, e
	}
	models, e := decode[[]WikiModel](s.Client.json(ctx, "GET", "/models", nil))
	if e != nil {
		return nil, e
	}
	out := []WikiModel{}
	for _, m := range models {
		if m.Type == "KnowledgeQA" {
			out = append(out, m)
		}
	}
	return out, nil
}

type WikiSettings struct {
	Enabled                bool    `json:"enabled"`
	Model                  string  `json:"model"`
	Revision               int64   `json:"revision"`
	Granularity            *string `json:"extraction_granularity,omitempty"`
	MaxPages               *int    `json:"max_pages_per_ingest,omitempty"`
	ContentInstructions    *string `json:"content_instructions,omitempty"`
	ExtractionInstructions *string `json:"extraction_instructions,omitempty"`
}

func (s *Service) ConfigureWiki(ctx context.Context, kb, actor string, in WikiSettings) error {
	if in.Revision < 1 || !in.validDepth() {
		return ErrInput
	}
	models, e := s.WikiModels(ctx, kb)
	if e != nil {
		return e
	}
	if in.Enabled {
		found := false
		for _, m := range models {
			found = found || m.ID == in.Model
		}
		if !found {
			return ErrInput
		}
	}
	// Serialize portal settings edits and preserve all upstream parser/index flags.
	return s.DB.TransactCtx(ctx, func(ctx context.Context, tx sqlx.Session) error {
		var b Base
		if e := tx.QueryRowCtx(ctx, &b, "SELECT id,name,description,enabled,revision FROM ai_knowledge_base WHERE id=? FOR UPDATE", kb); e != nil {
			return e
		}
		if b.Revision != in.Revision {
			return ErrConflict
		}
		engine, e := decode[map[string]json.RawMessage](s.Client.json(ctx, "GET", "/knowledge-bases/"+kb, nil))
		if e != nil {
			return e
		}
		var strategy map[string]bool
		_ = json.Unmarshal(engine["indexing_strategy"], &strategy)
		if strategy == nil {
			return ErrUnavailable
		}
		strategy["wiki_enabled"] = in.Enabled
		var wiki map[string]any
		_ = json.Unmarshal(engine["wiki_config"], &wiki)
		if wiki == nil {
			wiki = map[string]any{}
		}
		if in.Enabled {
			wiki["synthesis_model_id"] = in.Model
		}
		applyWikiDepth(wiki, in)
		cfg := map[string]any{"indexing_strategy": strategy, "wiki_config": wiki}
		for _, key := range []string{"chunking_config", "image_processing_config", "faq_config", "auto_tag_config", "profile_config"} {
			if v, ok := engine[key]; ok {
				cfg[key] = v
			}
		}
		if _, e = s.Client.json(ctx, "PUT", "/knowledge-bases/"+kb, map[string]any{"name": engine["name"], "description": engine["description"], "config": cfg}); e != nil {
			return e
		}
		if _, e = tx.ExecCtx(ctx, "UPDATE ai_knowledge_base SET revision=revision+1 WHERE id=?", kb); e != nil {
			return e
		}
		return s.audit(ctx, tx, actor, "configure_wiki", kb)
	})
}

// Absent fields preserve existing engine settings, including old clients that
// only change the model or enable switch. Limits bound per-document model work.
func (in WikiSettings) validDepth() bool {
	if in.Granularity != nil && *in.Granularity != "focused" && *in.Granularity != "standard" && *in.Granularity != "exhaustive" {
		return false
	}
	if in.MaxPages != nil && (*in.MaxPages < 1 || *in.MaxPages > 32) {
		return false
	}
	for _, v := range []*string{in.ContentInstructions, in.ExtractionInstructions} {
		if v != nil && (len([]rune(*v)) > 4000 || strings.ContainsRune(*v, '\x00')) {
			return false
		}
	}
	return true
}
func applyWikiDepth(wiki map[string]any, in WikiSettings) {
	defaults := map[string]any{
		"max_pages_per_ingest": 12, "extraction_granularity": "standard",
		"ingest_batch_size": 2, "ingest_map_parallel": 1, "ingest_reduce_parallel": 1, "ingest_max_inflight": 1,
		"content_instructions": "用中文整理文献知识，保留原文来源。古籍观点标为历史文化观点，不作为已证实事实；不得根据外貌推断人格、命运或健康。缺失证据不补造，矛盾观点并列呈现。",
	}
	for k, v := range defaults {
		if _, ok := wiki[k]; !ok {
			wiki[k] = v
		}
	}
	if in.Granularity != nil {
		wiki["extraction_granularity"] = *in.Granularity
	}
	if in.MaxPages != nil {
		wiki["max_pages_per_ingest"] = *in.MaxPages
	}
	if in.ContentInstructions != nil {
		wiki["content_instructions"] = *in.ContentInstructions
	}
	if in.ExtractionInstructions != nil {
		wiki["extraction_instructions"] = *in.ExtractionInstructions
	}
}

func wikiSlug(slug string) (string, error) {
	if len(slug) == 0 || len(slug) > 1024 || strings.ContainsAny(slug, "\\%?#\x00\r\n") {
		return "", ErrInput
	}
	parts := strings.Split(slug, "/")
	for i, p := range parts {
		if p == "" || p == "." || p == ".." {
			return "", ErrInput
		}
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/"), nil
}
func (s *Service) WikiRead(ctx context.Context, kb, kind, slug, query string, page int) (any, error) {
	if page < 1 || page > 10000 || len([]rune(query)) > 200 {
		return nil, ErrInput
	}
	var suffix string
	var result any
	switch kind {
	case "lint":
		suffix = "lint"
		result = &WikiLint{}
	case "issues":
		suffix = "issues?status=pending"
		result = &[]WikiIssue{}
	case "revisions":
		escaped, e := wikiSlug(slug)
		if e != nil {
			return nil, e
		}
		suffix = "revisions/" + escaped + "?limit=20&offset=" + strconv.Itoa((page-1)*20)
		result = &WikiHistory{}
	case "revision":
		escaped, e := wikiSlug(slug)
		if e != nil {
			return nil, e
		}
		version, e := strconv.Atoi(query)
		if e != nil || version < 1 {
			return nil, ErrInput
		}
		suffix = "revisions/" + escaped + "?version=" + strconv.Itoa(version)
		result = &WikiRevision{}
	case "stats":
		suffix = "stats"
		result = &WikiStats{}
	case "graph":
		suffix = "graph?mode=overview&limit=120"
		result = &WikiGraph{}
	case "pages":
		suffix = "pages?page=" + strconv.Itoa(page) + "&page_size=20&query=" + url.QueryEscape(query)
		result = &struct {
			Pages []WikiPage `json:"pages"`
			Total int        `json:"total"`
		}{}
	case "page":
		escaped, e := wikiSlug(slug)
		if e != nil {
			return nil, e
		}
		suffix = "pages/" + escaped
		result = &WikiPage{}
	default:
		return nil, ErrInput
	}
	state, e := s.WikiStatus(ctx, kb)
	if e != nil {
		return nil, e
	}
	if !state.Indexing.Wiki {
		return nil, ErrInput
	}
	raw, e := s.Client.raw(ctx, "GET", "/knowledgebase/"+kb+"/wiki/"+suffix, nil, "")
	if e != nil {
		return nil, e
	}
	if json.Unmarshal(raw, result) != nil {
		return nil, ErrUnavailable
	}
	if p, ok := result.(*WikiPage); ok {
		policies, err := s.policies(ctx, kb)
		if err != nil {
			return nil, err
		}
		p.References = []WikiReference{}
		for _, ref := range p.Sources {
			id := strings.SplitN(ref, "|", 2)[0]
			if policy, ok := policies[id]; ok {
				p.References = append(p.References, WikiReference{ID: id, Source: policy.Source, Approved: policy.Enabled})
			}
		}
	}
	return result, nil
}

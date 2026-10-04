package weknora

import (
	"context"
	"encoding/json"
	"github.com/askxuan/ai-service/internal/knowledge"
	"math"
	"net/url"
	"strings"
	"time"
)

type EngineModel struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Type        string `json:"type"`
	Source      string `json:"source"`
	Status      string `json:"status"`
	Builtin     bool   `json:"is_builtin"`
	ManagedBy   string `json:"managed_by"`
	Parameters  struct {
		BaseURL   string `json:"base_url"`
		Provider  string `json:"provider"`
		Embedding struct {
			Dimension int `json:"dimension"`
		} `json:"embedding_parameters"`
	} `json:"parameters"`
}
type ModelInput struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Type        string `json:"type"`
	BaseURL     string `json:"baseUrl"`
	APIKey      string `json:"apiKey"`
	Provider    string `json:"provider"`
	Dimension   int    `json:"dimension"`
}

func (s *Service) Models(ctx context.Context) ([]EngineModel, error) {
	return decode[[]EngineModel](s.Client.json(ctx, "GET", "/models", nil))
}
func (s *Service) engineModel(ctx context.Context, id string) (EngineModel, error) {
	if !validModelID(id) {
		return EngineModel{}, ErrInput
	}
	return decode[EngineModel](s.Client.json(ctx, "GET", "/models/"+id, nil))
}
func validModelID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, r := range id {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}
func validateModel(v ModelInput) error {
	if strings.TrimSpace(v.Name) == "" || len(v.Name) > 200 || len(v.DisplayName) > 200 || len(v.APIKey) > 4096 {
		return ErrInput
	}
	if v.Type != "Embedding" && v.Type != "Rerank" && v.Type != "KnowledgeQA" {
		return ErrInput
	}
	if v.Provider != "openai" && v.Provider != "generic" && v.Provider != "aliyun" {
		return ErrInput
	}
	u, e := url.Parse(v.BaseURL)
	if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Port() != "" && u.Port() != "443" || strings.Contains(u.Path, "..") {
		return ErrInput
	}
	if v.Type == "Embedding" && (v.Dimension < 1 || v.Dimension > 8192) {
		return ErrInput
	}
	return nil
}

// Endpoint allow-list and DNS validation are also enforced by WeKnora. No arbitrary proxy is exposed.
func (s *Service) SaveModel(ctx context.Context, id, actor string, v ModelInput) (EngineModel, error) {
	if e := validateModel(v); e != nil {
		return EngineModel{}, e
	}
	if id != "" {
		m, e := s.engineModel(ctx, id)
		if e != nil {
			return m, e
		}
		if m.Builtin || m.ManagedBy == "yaml" || m.Type != v.Type || strings.TrimRight(m.Parameters.BaseURL, "/") != strings.TrimRight(v.BaseURL, "/") || m.Parameters.Provider != v.Provider {
			return m, ErrConflict
		}
		if e = s.modelUnused(ctx, id); e != nil {
			return m, e
		}
	}
	params := map[string]any{"base_url": strings.TrimRight(v.BaseURL, "/"), "provider": v.Provider, "embedding_parameters": map[string]int{"dimension": v.Dimension}, "max_concurrency": 1}
	if v.APIKey != "" {
		params["api_key"] = v.APIKey
	}
	payload := map[string]any{"name": v.Name, "display_name": v.DisplayName, "type": v.Type, "source": "remote", "parameters": params}
	method, path := "POST", "/models"
	if id != "" {
		method = "PUT"
		path += "/" + id
	}
	out, e := decode[EngineModel](s.Client.json(ctx, method, path, payload))
	if e != nil {
		return out, e
	}
	if id != "" && v.APIKey != "" {
		if _, e = s.Client.json(ctx, "PUT", "/models/"+id+"/credentials", map[string]string{"api_key": v.APIKey}); e != nil {
			return out, e
		}
	}
	e = s.audit(ctx, s.DB, actor, "save_engine_model", out.ID)
	return out, e
}
func (s *Service) modelUnused(ctx context.Context, id string) error {
	bases, e := s.Bases(ctx)
	if e != nil {
		return e
	}
	for _, b := range bases {
		v, e := decode[map[string]json.RawMessage](s.Client.json(ctx, "GET", "/knowledge-bases/"+b.ID, nil))
		if e != nil {
			return e
		}
		for _, key := range []string{"embedding_model_id", "summary_model_id", "wiki_config"} {
			if strings.Contains(string(v[key]), `"`+id+`"`) {
				return ErrConflict
			}
		}
	}
	// Published versions are immutable and may be rolled back. Never mutate their dependencies.
	var n int64
	if e = s.DB.QueryRowCtx(ctx, &n, "SELECT (SELECT COUNT(*) FROM ai_agent_version WHERE definition_json LIKE ?) + (SELECT COUNT(*) FROM ai_agent_workspace WHERE draft_json LIKE ?)", "%\""+id+"\"%", "%\""+id+"\"%"); e != nil {
		return e
	}
	if n > 0 {
		return ErrConflict
	}
	return nil
}
func (s *Service) DeleteModel(ctx context.Context, id, actor string) error {
	m, e := s.engineModel(ctx, id)
	if e != nil {
		return e
	}
	if m.Builtin || m.ManagedBy == "yaml" || id == s.Client.embedding {
		return ErrConflict
	}
	if e = s.modelUnused(ctx, id); e != nil {
		return e
	}
	_, e = s.Client.json(ctx, "DELETE", "/models/"+id, nil)
	if e != nil {
		return e
	}
	return s.audit(ctx, s.DB, actor, "delete_engine_model", id)
}

type DebugResult struct {
	OK           bool  `json:"ok"`
	Elapsed      int64 `json:"elapsed_ms"`
	Observations struct {
		Dimension int `json:"dimension"`
		Count     int `json:"result_count"`
	} `json:"observations"`
	Raw json.RawMessage `json:"raw_response,omitempty"`
}

func (s *Service) debugModel(ctx context.Context, id, input string, documents []string) (DebugResult, error) {
	if !validModelID(id) || len([]rune(input)) > 4000 || len(documents) > 40 {
		return DebugResult{}, ErrInput
	}
	b, _ := json.Marshal(documents)
	form := url.Values{"input": {input}, "documents": {string(b)}, "options": {`{"max_tokens":64}`}}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	v, e := decode[DebugResult](s.Client.call(ctx, "POST", "/models/"+id+"/debug", strings.NewReader(form.Encode()), "application/x-www-form-urlencoded"))
	if e == nil && !v.OK {
		e = ErrUnavailable
	}
	return v, e
}
func (s *Service) TestModel(ctx context.Context, id string) (DebugResult, error) {
	v, e := s.debugModel(ctx, id, "如何核对文献出处？", []string{"引用需要注明文献名称和原文位置。", "今天晚餐吃什么。"})
	v.Raw = nil
	return v, e
}
func (s *Service) rerank(ctx context.Context, id, q string, hits []searchHit) ([]searchHit, error) {
	m, e := s.engineModel(ctx, id)
	if e != nil {
		return nil, e
	}
	if m.Type != "Rerank" {
		return nil, ErrInput
	}
	docs := make([]string, len(hits))
	for i, h := range hits {
		docs[i] = h.Content
	}
	v, e := s.debugModel(ctx, id, q, docs)
	if e != nil {
		return nil, e
	}
	var rows []struct {
		Index int     `json:"index"`
		Score float64 `json:"relevance_score"`
	}
	if json.Unmarshal(v.Raw, &rows) != nil || len(rows) != len(hits) {
		return nil, ErrUnavailable
	}
	out := make([]searchHit, 0, len(rows))
	seen := map[int]bool{}
	for _, r := range rows {
		if r.Index < 0 || r.Index >= len(hits) || seen[r.Index] || math.IsNaN(r.Score) || math.IsInf(r.Score, 0) {
			return nil, ErrUnavailable
		}
		seen[r.Index] = true
		h := hits[r.Index]
		h.Score = r.Score
		out = append(out, h)
	}
	return out, nil
}

func (s *Service) ValidateRetrieval(ctx context.Context, p knowledge.RetrievalPolicy, ids []string) error {
	if !p.Valid() {
		return ErrInput
	}
	if p.RerankModel != "" {
		m, e := s.engineModel(ctx, p.RerankModel)
		if e != nil {
			return e
		}
		if m.Type != "Rerank" {
			return ErrInput
		}
	}
	if p.GraphEnabled {
		if _, e := s.graphQuery(ctx, "RETURN 1", nil); e != nil {
			return e
		}
	}
	for _, id := range ids {
		if _, e := s.base(ctx, id); e != nil {
			return e
		}
	}
	return nil
}

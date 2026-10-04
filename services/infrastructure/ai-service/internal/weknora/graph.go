package weknora

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type EntityNode struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Document   string   `json:"document"`
	Chunks     []string `json:"chunks"`
	Attributes []string `json:"attributes"`
	Source     string   `json:"source"`
}
type EntityEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Name   string `json:"name"`
}
type EntityGraph struct {
	Nodes     []EntityNode `json:"nodes"`
	Edges     []EntityEdge `json:"edges"`
	Truncated bool         `json:"truncated"`
	Engine    string       `json:"engine"`
}

func (s *Service) graphQuery(ctx context.Context, statement string, params map[string]any) ([][]json.RawMessage, error) {
	endpoint, key := os.Getenv("AI_NEO4J_HTTP"), os.Getenv("AI_NEO4J_PASSWORD")
	if endpoint == "" || key == "" {
		return nil, ErrUnavailable
	}
	b, _ := json.Marshal(map[string]any{"statements": []any{map[string]any{"statement": statement, "parameters": params}}})
	req, e := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(endpoint, "/")+"/db/neo4j/tx/commit", bytes.NewReader(b))
	if e != nil {
		return nil, e
	}
	req.SetBasicAuth("neo4j", key)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, e := client.Do(req)
	if e != nil {
		return nil, ErrUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, ErrUnavailable
	}
	var result struct {
		Results []struct {
			Data []struct {
				Row []json.RawMessage `json:"row"`
			} `json:"data"`
		} `json:"results"`
		Errors []json.RawMessage `json:"errors"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 2*1024*1024)).Decode(&result) != nil || len(result.Errors) > 0 || len(result.Results) != 1 {
		return nil, ErrUnavailable
	}
	rows := [][]json.RawMessage{}
	for _, r := range result.Results[0].Data {
		rows = append(rows, r.Row)
	}
	return rows, nil
}
func (s *Service) EntityGraph(ctx context.Context, kb, q string) (EntityGraph, error) {
	return s.entityGraph(ctx, kb, q, nil)
}
func (s *Service) entityGraph(ctx context.Context, kb, q string, seeds []string) (EntityGraph, error) {
	out := EntityGraph{Engine: "Neo4j", Nodes: []EntityNode{}, Edges: []EntityEdge{}}
	base, e := s.base(ctx, kb)
	if e != nil {
		return out, e
	}
	if !base.Enabled || len([]rune(q)) > 80 {
		return out, ErrInput
	}
	policies, e := s.policies(ctx, kb)
	if e != nil {
		return out, e
	}
	docs := []string{}
	for id, p := range policies {
		if p.Enabled {
			docs = append(docs, id)
		}
	}
	if len(docs) == 0 {
		return out, nil
	}
	// The sole interpolated label comes from a registered UUID; all user text is parameterized.
	label := "ENTITY" + strings.ReplaceAll(kb, "-", "_")
	statement := "MATCH (n:`" + label + "`) WHERE n.kg IN $docs AND ($q = '' OR n.name CONTAINS $q) AND (size($seeds)=0 OR any(c IN coalesce(n.chunks,[]) WHERE c IN $seeds)) WITH n ORDER BY n.name LIMIT 100 OPTIONAL MATCH (n)-[r]-(m:`" + label + "`) WHERE m.kg IN $docs RETURN {id:elementId(n),name:n.name,document:n.kg,chunks:n.chunks,attributes:n.attributes}, CASE WHEN m IS NULL THEN null ELSE {id:elementId(m),name:m.name,document:m.kg,chunks:m.chunks,attributes:m.attributes} END, CASE WHEN r IS NULL THEN null ELSE {source:elementId(startNode(r)),target:elementId(endNode(r)),name:type(r)} END LIMIT 301"
	rows, e := s.graphQuery(ctx, statement, map[string]any{"docs": docs, "q": strings.TrimSpace(q), "seeds": seedsOrEmpty(seeds)})
	if e != nil {
		return out, e
	}
	out.Truncated = len(rows) > 300
	if out.Truncated {
		rows = rows[:300]
	}
	// Revocation during graph I/O must remove the affected entities and edges.
	current, e := s.policies(ctx, kb)
	if e != nil {
		return out, e
	}
	base, e = s.base(ctx, kb)
	if e != nil || !base.Enabled {
		return out, ErrInput
	}
	nodes := map[string]bool{}
	for _, row := range rows {
		if len(row) != 3 {
			continue
		}
		for _, raw := range row[:2] {
			var n EntityNode
			if json.Unmarshal(raw, &n) == nil && n.ID != "" && current[n.Document].Enabled && !nodes[n.ID] {
				n.Source = current[n.Document].Source
				nodes[n.ID] = true
				out.Nodes = append(out.Nodes, n)
			}
		}
	}
	edges := map[string]bool{}
	for _, row := range rows {
		if len(row) != 3 {
			continue
		}
		var edge EntityEdge
		if json.Unmarshal(row[2], &edge) == nil && nodes[edge.Source] && nodes[edge.Target] {
			key := edge.Source + "\x00" + edge.Target + "\x00" + edge.Name
			if !edges[key] {
				edges[key] = true
				out.Edges = append(out.Edges, edge)
			}
		}
	}
	return out, nil
}

type GraphSettings struct {
	Model    string `json:"model"`
	Enabled  bool   `json:"enabled"`
	Revision int64  `json:"revision"`
}

func (s *Service) ConfigureGraph(ctx context.Context, kb, actor string, in GraphSettings) error {
	if in.Revision < 1 || !validID(kb) {
		return ErrInput
	}
	if in.Enabled {
		if _, e := s.graphQuery(ctx, "RETURN 1", nil); e != nil {
			return e
		}
	}
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
		var model string
		_ = json.Unmarshal(engine["summary_model_id"], &model)
		if in.Model != "" && in.Model != model {
			m, err := s.engineModel(ctx, in.Model)
			if err != nil || m.Type != "KnowledgeQA" {
				return ErrInput
			}
			if err = s.bindSummaryModel(ctx, kb, in.Model, engine); err != nil {
				return err
			}
			model = in.Model
		}
		if in.Enabled && model == "" {
			return ErrInput
		}
		cfg := map[string]any{}
		for _, key := range []string{"chunking_config", "image_processing_config", "faq_config", "auto_tag_config", "profile_config", "wiki_config"} {
			if v, ok := engine[key]; ok {
				cfg[key] = v
			}
		}
		var strategy map[string]any
		if json.Unmarshal(engine["indexing_strategy"], &strategy) != nil || strategy == nil {
			return ErrUnavailable
		}
		strategy["graph_enabled"] = in.Enabled
		cfg["indexing_strategy"] = strategy
		if _, e = s.Client.json(ctx, "PUT", "/knowledge-bases/"+kb, map[string]any{"name": engine["name"], "description": engine["description"], "config": cfg}); e != nil {
			return e
		}
		if _, e = tx.ExecCtx(ctx, "UPDATE ai_knowledge_base SET revision=revision+1 WHERE id=?", kb); e != nil {
			return e
		}
		return s.audit(ctx, tx, actor, "configure_graph", kb)
	})
}

func seedsOrEmpty(v []string) []string {
	if v == nil {
		return []string{}
	}
	return v
}

// The supported initialization API owns model bindings. Round-trip its parser
// configuration instead of clearing settings when the summary slot changes.
func (s *Service) bindSummaryModel(ctx context.Context, kb, id string, engine map[string]json.RawMessage) error {
	old, e := decode[map[string]json.RawMessage](s.Client.json(ctx, "GET", "/initialization/config/"+kb, nil))
	if e != nil {
		return e
	}
	payload := map[string]any{"llmModelId": id, "embeddingModelId": engine["embedding_model_id"]}
	for _, k := range []string{"documentSplitting", "nodeExtract", "questionGeneration", "multimodal"} {
		if v, ok := old[k]; ok {
			payload[k] = v
		}
	}
	for _, k := range []string{"vlm_config", "asr_config"} {
		if v, ok := engine[k]; ok {
			payload[k] = v
		}
	}
	for a, b := range map[string]string{"storage_provider": "storageProvider", "storage_backend_id": "storageBackendId"} {
		if v, ok := engine[a]; ok {
			payload[b] = v
		}
	}
	_, e = s.Client.json(ctx, "PUT", "/initialization/config/"+kb, payload)
	return e
}

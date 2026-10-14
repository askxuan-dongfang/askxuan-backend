package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/askxuan/ai-service/internal/settings"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/askxuan/ai-service/internal/weknora"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
)

type libraryItem struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Model           string            `json:"model"`
	Provider        string            `json:"provider"`
	BaseURL         string            `json:"baseUrl"`
	Kind            string            `json:"kind"`
	Source          string            `json:"source"`
	Dimension       int               `json:"dimension"`
	ContextWindow   int               `json:"contextWindow"`
	MaxOutputTokens int               `json:"maxOutputTokens"`
	Bindings        map[string]string `json:"bindings"`
}

func registerModelLibrary(server *rest.Server, s *svc.ServiceContext) {
	for _, op := range []struct{ method, path, action string }{{"GET", "", "list"}, {"POST", "/language", "add"}, {"POST", "/language/test", "test-input"}, {"POST", "/:model/test", "test"}, {"DELETE", "/:model", "delete"}, {"PUT", "/uses", "uses"}} {
		server.AddRoute(rest.Route{Method: op.method, Path: "/api/v1/ai/admin/model-library" + op.path, Handler: modelLibraryHandler(s, op.action)})
	}
}

func modelLibraryHandler(s *svc.ServiceContext, action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		actor := r.Header.Get("X-User-Id")
		id, err := strconv.ParseInt(actor, 10, 64)
		super := false
		for _, role := range strings.Split(r.Header.Get("X-User-Roles"), ",") {
			super = super || role == "platform_super"
		}
		if err != nil || id <= 0 || !super || r.Header.Get("X-User-Type") != "admin" {
			common.JsonError(w, common.ErrRoleForbidden)
			return
		}
		if s.Settings == nil {
			common.JsonError(w, common.NewBizError(50301, "模型库暂不可用"))
			return
		}
		finish := func(value any, err error) {
			if err == nil {
				common.Ok(w, value)
				return
			}
			code := 40001
			message := err.Error()
			if errors.Is(err, settings.ErrConflict) || errors.Is(err, weknora.ErrConflict) {
				code = 40901
			}
			if errors.Is(err, weknora.ErrUnavailable) {
				code, message = 50301, "知识模型连接不可用，请检查模型与知识引擎出站配置"
			}
			common.JsonError(w, common.NewBizError(code, message))
		}
		read := func(v any) bool {
			d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16384))
			d.DisallowUnknownFields()
			var extra any
			if d.Decode(v) != nil || d.Decode(&extra) != io.EOF {
				common.JsonError(w, common.ErrParam)
				return false
			}
			return true
		}
		var path struct {
			Model string `path:"model,optional"`
		}
		_ = httpx.ParsePath(r, &path)
		ctx := r.Context()
		switch action {
		case "list":
			items := []libraryItem{}
			warnings := []string{}
			language, err := s.Settings.LanguageModels(ctx)
			if err != nil {
				warnings = append(warnings, "原有大模型连接暂不可用；已独立登记的模型仍可选择")
			}
			linked := map[string]bool{}
			for _, m := range language {
				document := m.DocumentModelID
				if document == "" {
					document = "chat:" + m.ID
				} else {
					linked[document] = true
				}
				bindings := map[string]string{"chat": m.ID, "document": document}
				if m.SupportsVision {
					bindings["vision"] = m.ID
				}
				items = append(items, libraryItem{ID: m.ID, Name: m.Name, Model: m.Model, Provider: m.Provider, BaseURL: m.BaseURL, Kind: "language", Source: m.Source, ContextWindow: m.ContextWindow, MaxOutputTokens: m.MaxOutputTokens, Bindings: bindings})
			}
			if s.WeKnora != nil {
				models, err := s.WeKnora.Models(ctx)
				if err != nil {
					warnings = append(warnings, "知识引擎暂不可用，知识模型未能加载")
				}
				for _, m := range models {
					if linked[m.ID] {
						continue
					}
					role := map[string]string{"Embedding": "embedding", "Rerank": "rerank", "KnowledgeQA": "document"}[m.Type]
					if role == "" {
						continue
					}
					name := m.DisplayName
					if name == "" {
						name = m.Name
					}
					source := "knowledge"
					if m.Builtin || m.ManagedBy == "yaml" {
						source = "builtin"
					}
					items = append(items, libraryItem{ID: m.ID, Name: name, Model: m.Name, Provider: m.Parameters.Provider, BaseURL: m.Parameters.BaseURL, Kind: m.Type, Source: source, Dimension: m.Parameters.Embedding.Dimension, Bindings: map[string]string{role: m.ID}})
				}
			} else {
				warnings = append(warnings, "知识引擎尚未配置")
			}
			uses, revision := s.Settings.ModelUses()
			finish(map[string]any{"list": items, "uses": uses, "revision": revision, "writable": s.Settings.Public().Writable, "defaultChat": s.Settings.Snapshot().Provider.Model(), "warnings": warnings}, nil)
		case "test-input", "add":
			var in settings.LanguageModelInput
			if !read(&in) {
				return
			}
			if action == "test-input" {
				v, err := s.Settings.TestLanguageModel(ctx, in)
				finish(v, err)
			} else {
				v, err := s.Settings.AddLanguageModel(ctx, in, actor)
				finish(v, err)
			}
		case "test":
			m, connection, err := s.Settings.LanguageConnection(ctx, path.Model)
			if err != nil {
				finish(nil, err)
				return
			}
			_, err = s.Settings.TestLanguageModel(ctx, settings.LanguageModelInput{LanguageModelValues: m.LanguageModelValues, APIKey: connection.APIKey})
			finish(map[string]bool{"ok": err == nil}, err)
		case "uses":
			var in struct {
				Uses     settings.ModelUses `json:"uses"`
				Revision int64              `json:"revision"`
			}
			if !read(&in) {
				return
			}
			if _, revision := s.Settings.ModelUses(); revision != in.Revision {
				finish(nil, settings.ErrConflict)
				return
			}
			for _, selected := range []struct{ id, kind string }{{in.Uses.Embedding, "Embedding"}, {in.Uses.Rerank, "Rerank"}} {
				if selected.id == "" {
					continue
				}
				if s.WeKnora == nil {
					finish(nil, weknora.ErrUnavailable)
					return
				}
				if err := s.WeKnora.ValidateModelType(ctx, selected.id, selected.kind); err != nil {
					finish(nil, err)
					return
				}
			}
			if in.Uses.Document != "" {
				in.Uses.Document, in.Revision, err = resolveDocumentModel(s, r, in.Uses.Document, actor, in.Revision)
				if err != nil {
					finish(nil, err)
					return
				}
			}
			finish(nil, s.Settings.SaveModelUses(ctx, in.Uses, in.Revision, actor))
		case "delete":
			var in struct {
				Revision int64 `json:"revision"`
			}
			if !read(&in) {
				return
			}
			if s.DB == nil {
				finish(nil, errors.New("引用校验暂不可用"))
				return
			}
			var n int64
			if err := s.DB.QueryRowCtx(ctx, &n, "SELECT (SELECT COUNT(*) FROM ai_agent_version WHERE definition_json LIKE ?) + (SELECT COUNT(*) FROM ai_agent_workspace WHERE draft_json LIKE ?)", "%\""+path.Model+"\"%", "%\""+path.Model+"\"%"); err != nil {
				finish(nil, errors.New("引用校验失败"))
				return
			}
			if n > 0 {
				finish(nil, errors.New("智能体草稿或已发布版本正在使用此模型"))
				return
			}
			m, _, err := s.Settings.LanguageConnection(ctx, path.Model)
			if err != nil {
				finish(nil, err)
				return
			}
			if m.DocumentModelID != "" {
				finish(nil, errors.New("此模型已用于文档理解，请保留知识模型与回滚依赖"))
				return
			}
			finish(nil, s.Settings.DeleteLanguageModel(path.Model, actor, in.Revision))
		}
	}
}

var documentProvision sync.Mutex

// Only a user's explicit document-model selection provisions an engine adapter.
// Keys travel server-to-server; the original chat connection is never rewritten.
func resolveDocumentModel(s *svc.ServiceContext, r *http.Request, selected, actor string, revision int64) (string, int64, error) {
	if s.WeKnora == nil {
		return "", revision, weknora.ErrUnavailable
	}
	if !strings.HasPrefix(selected, "chat:") {
		return selected, revision, s.WeKnora.ValidateModelType(r.Context(), selected, "KnowledgeQA")
	}
	if s.Settings == nil {
		return "", revision, errors.New("模型库暂不可用")
	}
	documentProvision.Lock()
	defer documentProvision.Unlock()
	m, connection, err := s.Settings.LanguageConnection(r.Context(), strings.TrimPrefix(selected, "chat:"))
	if err != nil {
		return "", revision, err
	}
	if m.DocumentModelID != "" {
		return m.DocumentModelID, revision, s.WeKnora.ValidateModelType(r.Context(), m.DocumentModelID, "KnowledgeQA")
	}
	if _, current := s.Settings.ModelUses(); current != revision {
		return "", revision, settings.ErrConflict
	}
	engine, err := s.WeKnora.SaveModel(r.Context(), "", actor, weknora.ModelInput{Name: m.Model, DisplayName: m.Name, Type: "KnowledgeQA", BaseURL: connection.BaseURL, APIKey: connection.APIKey, Provider: "openai"})
	if err != nil {
		return "", revision, weknora.ErrUnavailable
	}
	_, err = s.WeKnora.TestModel(r.Context(), engine.ID)
	if err == nil {
		err = s.Settings.LinkKnowledgeModel(m.ID, engine.ID, actor, revision)
	}
	if err != nil {
		cleanup, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = s.WeKnora.DeleteModel(cleanup, engine.ID, actor)
		return "", revision, err
	}
	return engine.ID, revision + 1, nil
}

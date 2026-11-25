package handler

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"

	"github.com/askxuan/ai-service/internal/provider"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/askxuan/ai-service/internal/weknora"
	"github.com/google/uuid"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func documentCompletionHandler(s *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.WeKnora == nil || s.Settings == nil {
			http.NotFound(w, r)
			return
		}
		var path struct {
			Model string `path:"model"`
		}
		if err := httpx.ParsePath(r, &path); err != nil {
			http.Error(w, "Invalid document model", http.StatusBadRequest)
			return
		}
		_, token := s.WeKnora.Client.DocumentConnection(path.Model)
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+token)) != 1 {
			http.Error(w, "Invalid document completion credential", http.StatusUnauthorized)
			return
		}

		routes := s.Settings.Snapshot().Provider
		if !provider.IsRegisteredModel(routes, path.Model) {
			http.NotFound(w, r)
			return
		}
		ctx := r.Context()
		p, selection, err := provider.ResolveProvider(ctx, routes, provider.Request{Model: path.Model})
		llm, ok := p.(*provider.OpenAICompatible)
		if err != nil || !ok {
			http.NotFound(w, r)
			return
		}
		var input struct {
			Model string `json:"model"`
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, weknora.MaxFileBytes))
		if err != nil || json.Unmarshal(body, &input) != nil || input.Model != selection.Model {
			http.Error(w, "Invalid document completion request", http.StatusBadRequest)
			return
		}

		// Each knowledge task call is a separate completion request.
		reply, err := llm.CompleteDocument(ctx, body, uuid.NewString())
		if err != nil {
			http.Error(w, "Document model is unavailable", http.StatusBadGateway)
			return
		}
		defer reply.Body.Close()
		if reply.StatusCode < 200 || reply.StatusCode >= 300 {
			http.Error(w, "Document model rejected the request", reply.StatusCode)
			return
		}
		w.Header().Set("Content-Type", reply.Header.Get("Content-Type"))
		w.WriteHeader(reply.StatusCode)
		_, _ = io.Copy(w, reply.Body)
	}
}

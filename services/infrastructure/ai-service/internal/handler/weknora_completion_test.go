package handler

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/askxuan/ai-service/internal/config"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/askxuan/ai-service/internal/settings"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/askxuan/ai-service/internal/weknora"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/rest/pathvar"
)

type documentTransport func(*http.Request) (*http.Response, error)

func (f documentTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDocumentReusesRegisteredModel(t *testing.T) {
	for _, tc := range []struct{ kind, baseURL string }{
		{provider.OpenCodeGo, provider.OpenCodeGoBaseURL},
		{"openai_compatible", "https://models.example.com/v1"},
	} {
		t.Run(tc.kind, func(t *testing.T) { testDocumentReusesRegisteredModel(t, tc.kind, tc.baseURL) })
	}
}

func testDocumentReusesRegisteredModel(t *testing.T, kind, baseURL string) {
	t.Helper()
	const libraryID = "registered-document-model"
	const engineID = "engine-document-model"
	const providerKey = "fixture-provider-key"
	const body = `{"model":"kimi-k3","messages":[{"role":"user","content":"Extract the release name."}],"response_format":{"type":"json_object"}}`
	dir := t.TempDir()
	key := make([]byte, 32)
	block, _ := aes.NewCipher(key)
	aead, _ := cipher.NewGCM(block)
	nonce := make([]byte, aead.NonceSize())
	record := fmt.Sprintf(`{"provider":"mock","defaultModel":"mock","revision":7,"models":[{"id":%q,"name":"Fixture LLM","model":"kimi-k3","provider":%q,"baseUrl":%q,"secret":%q}]}`,
		libraryID, kind, baseURL, providerKey)
	if err := os.WriteFile(filepath.Join(dir, "settings.enc"), aead.Seal(nonce, nonce, []byte(record), []byte("askxuan-ai-settings-v1")), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := settings.New(config.AIConf{Provider: "mock"}, dir, base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatal(err)
	}
	p, _, err := provider.ResolveProvider(context.Background(), m.Snapshot().Provider, provider.Request{Model: libraryID})
	if err != nil {
		t.Fatal(err)
	}
	var upstreamCalls atomic.Int32
	const completion = `{"choices":[{"message":{"content":"{\"release\":\"fixture\"}"},"finish_reason":"stop"}]}`
	p.(*provider.OpenAICompatible).SetHTTPClient(&http.Client{Transport: documentTransport(func(r *http.Request) (*http.Response, error) {
		upstreamCalls.Add(1)
		if r.URL.String() != baseURL+"/chat/completions" || r.Header.Get("Authorization") != "Bearer "+providerKey {
			t.Error("unexpected provider connection")
		}
		if session := r.Header.Get("x-opencode-session"); (session != "") != (kind == provider.OpenCodeGo) ||
			(kind == provider.OpenCodeGo && r.UserAgent() != "AskXuan/1.0") {
			t.Error("unexpected provider session metadata")
		}
		raw, _ := io.ReadAll(r.Body)
		if string(raw) != body {
			t.Error("engine completion request was changed")
		}
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(completion))}, nil
	})})
	s := &svc.ServiceContext{Settings: m}
	complete := func(model, token, payload string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(http.MethodPost, "/internal/weknora/models/"+model+"/v1/chat/completions", strings.NewReader(payload))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		documentCompletionHandler(s)(w, pathvar.WithVars(r, map[string]string{"model": model}))
		return w
	}
	var provisioned atomic.Int32
	var model struct {
		Name       string `json:"name"`
		Type       string `json:"type"`
		Parameters struct {
			BaseURL string `json:"base_url"`
			APIKey  string `json:"api_key"`
		} `json:"parameters"`
	}
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/models":
			if r.Method == http.MethodPost {
				provisioned.Add(1)
				if json.NewDecoder(r.Body).Decode(&model) != nil || model.Name != "kimi-k3" ||
					model.Type != "KnowledgeQA" || model.Parameters.APIKey == providerKey || model.Parameters.APIKey == "" ||
					model.Parameters.BaseURL != "http://askxuan-ai-service:8098/internal/weknora/models/"+libraryID+"/v1" {
					t.Error("invalid model provisioning")
				}
				_, _ = fmt.Fprintf(w, `{"success":true,"data":{"id":%q,"type":"KnowledgeQA"}}`, engineID)
			} else if provisioned.Load() > 0 {
				_, _ = fmt.Fprintf(w, `{"success":true,"data":[{"id":%q,"name":"kimi-k3","type":"KnowledgeQA"}]}`, engineID)
			} else {
				_, _ = w.Write([]byte(`{"success":true,"data":[]}`))
			}
		case "/api/v1/models/" + engineID:
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": map[string]any{
				"id": engineID, "type": "KnowledgeQA", "parameters": model.Parameters,
			}})
		case "/api/v1/models/" + engineID + "/debug":
			result := complete(libraryID, model.Parameters.APIKey, body)
			if result.Code != http.StatusOK || result.Body.String() != completion {
				t.Error("knowledge probe did not reach the selected registered provider")
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			_, _ = w.Write([]byte(`{"success":true,"data":{"ok":true}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer engine.Close()
	db, audit, _ := sqlmock.New()
	defer db.Close()
	audit.ExpectExec("INSERT INTO ai_knowledge_audit").WithArgs("1", "save_engine_model", engineID).WillReturnResult(sqlmock.NewResult(1, 1))
	client, _ := weknora.New(engine.URL, "engine-fixture-key", "embed")
	s.WeKnora = &weknora.Service{DB: sqlx.NewSqlConnFromDB(db), Client: client}

	_, token := client.DocumentConnection(libraryID)
	if complete(libraryID, providerKey, body).Code != http.StatusUnauthorized ||
		complete("another-model", token, body).Code != http.StatusUnauthorized ||
		complete(libraryID, token, `{"model":"different"}`).Code != http.StatusBadRequest || upstreamCalls.Load() != 0 {
		t.Fatal("invalid model completion reached the provider")
	}

	assertBinding := func(want string) {
		t.Helper()
		w := httptest.NewRecorder()
		modelLibraryHandler(s, "list")(w, libraryRequest("GET", ""))
		var reply struct {
			Data struct{ List []libraryItem }
		}
		if json.Unmarshal(w.Body.Bytes(), &reply) != nil || strings.Contains(w.Body.String(), providerKey) || strings.Contains(w.Body.String(), token) {
			t.Fatal("unexpected or unredacted model library")
		}
		for _, item := range reply.Data.List {
			if item.ID == engineID {
				t.Fatal("linked engine model duplicated the library record")
			}
			if item.ID == libraryID && item.Bindings["chat"] == libraryID && item.Bindings["document"] == want {
				return
			}
		}
		t.Fatal("registered model is missing a binding", want)
	}
	assertBinding("chat:" + libraryID)
	w := httptest.NewRecorder()
	choice := fmt.Sprintf(`{"revision":7,"uses":{"chat":%q,"document":%q}}`, libraryID, "chat:"+libraryID)
	modelLibraryHandler(s, "uses")(w, libraryRequest("PUT", choice))
	if !strings.Contains(w.Body.String(), `"code":0`) {
		t.Fatal(w.Body.String())
	}
	uses, revision := m.ModelUses()
	if uses.Chat != libraryID || uses.Document != engineID || revision != 9 || upstreamCalls.Load() != 1 {
		t.Fatal("model selection was not saved", uses, revision)
	}
	got, _, err := resolveDocumentModel(s, libraryRequest("POST", ""), "chat:"+libraryID, "1", revision)
	if err != nil || got != engineID || provisioned.Load() != 1 {
		t.Fatal("repeated selections did not reuse the engine model", got, err)
	}
	assertBinding(engineID)
	if err := audit.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

package handler

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
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
)

func libraryRequest(method, body string) *http.Request {
	r := httptest.NewRequest(method, "/api/v1/ai/admin/model-library", strings.NewReader(body))
	r.Header.Set("X-User-Id", "1")
	r.Header.Set("X-User-Type", "admin")
	r.Header.Set("X-User-Roles", "platform_super")
	return r
}

func TestLibraryDocumentConnectionReusedWithoutExposingKey(t *testing.T) {
	m, err := settings.New(config.AIConf{Provider: "volcengine", BaseURL: "https://ark.cn-beijing.volces.com/api/plan/v3", APIKey: "fixture-private-plan-key", Model: "ark-code-latest", MaxOutputTokens: 8192}, t.TempDir(), base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	var provisions atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/models" && r.Method == "POST" {
			provisions.Add(1)
			var payload struct {
				Name       string         `json:"name"`
				Type       string         `json:"type"`
				Parameters map[string]any `json:"parameters"`
			}
			_ = json.NewDecoder(r.Body).Decode(&payload)
			if payload.Name != "ark-code-latest" || payload.Type != "KnowledgeQA" || payload.Parameters["api_key"] != "fixture-private-plan-key" || payload.Parameters["base_url"] != "https://ark.cn-beijing.volces.com/api/plan/v3" {
				t.Error("connection was not reused server-side")
			}
			if payload.Parameters["extra_config"].(map[string]any)["thinking_control"] != "none" {
				t.Error("Plan override not disabled")
			}
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":"engine-doc","type":"KnowledgeQA"}}`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/debug") {
			_ = r.ParseForm()
			var options map[string]any
			_ = json.Unmarshal([]byte(r.Form.Get("options")), &options)
			if options["thinking"] != nil || options["max_tokens"] != float64(2048) {
				t.Error("Plan probe parameters invalid")
			}
			_, _ = w.Write([]byte(`{"success":true,"data":{"ok":true,"raw_response":"private diagnostic"}}`))
			return
		}
		if r.URL.Path == "/api/v1/models/engine-doc" {
			_, _ = w.Write([]byte(`{"success":true,"data":{"id":"engine-doc","name":"ark-code-latest","type":"KnowledgeQA","parameters":{"api_key":"fixture-private-plan-key","base_url":"https://ark.cn-beijing.volces.com/api/plan/v3"}}}`))
			return
		}
		if r.URL.Path == "/api/v1/models" {
			_, _ = w.Write([]byte(`{"success":true,"data":[{"id":"engine-doc","name":"ark-code-latest","type":"KnowledgeQA","parameters":{"api_key":"fixture-private-plan-key"}},{"id":"embed","name":"Embedding","type":"Embedding"},{"id":"rank","name":"Rerank","type":"Rerank"}]}`))
			return
		}
		t.Error("unexpected path", r.URL.Path)
		http.NotFound(w, r)
	}))
	defer srv.Close()
	db, mock, _ := sqlmock.New()
	defer db.Close()
	mock.ExpectExec("INSERT INTO ai_knowledge_audit").WithArgs("1", "save_engine_model", "engine-doc").WillReturnResult(sqlmock.NewResult(1, 1))
	c, _ := weknora.New(srv.URL, "engine-key", "embed")
	s := &svc.ServiceContext{Settings: m, WeKnora: &weknora.Service{DB: sqlx.NewSqlConnFromDB(db), Client: c}}
	r := libraryRequest("PUT", `{"revision":0,"uses":{"document":"chat:ark-code-latest"}}`)
	out := httptest.NewRecorder()
	modelLibraryHandler(s, "uses")(out, r)
	if !strings.Contains(out.Body.String(), `"code":0`) {
		t.Fatal(out.Body.String())
	}
	uses, revision := m.ModelUses()
	if revision != 2 || uses.Document != "engine-doc" {
		t.Fatal(uses, revision)
	}
	id, next, err := resolveDocumentModel(s, libraryRequest("PUT", ""), "chat:ark-code-latest", "1", revision)
	if err != nil || id != "engine-doc" || next != revision || provisions.Load() != 1 {
		t.Fatal("duplicate adapter created", id, next, err)
	}
	out = httptest.NewRecorder()
	modelLibraryHandler(s, "list")(out, libraryRequest("GET", ""))
	var response struct {
		Data struct {
			List []libraryItem `json:"list"`
		}
	}
	_ = json.Unmarshal(out.Body.Bytes(), &response)
	if len(response.Data.List) != 3 {
		t.Fatal("adapter duplicated in catalog", out.Body.String())
	}
	for _, item := range response.Data.List {
		if item.ID == "ark-code-latest" && (item.Bindings["document"] != "engine-doc" || item.Bindings["chat"] != "ark-code-latest") {
			t.Fatal(item)
		}
	}
	if strings.Contains(out.Body.String(), "fixture-private-plan-key") || strings.Contains(out.Body.String(), "private diagnostic") {
		t.Fatal("secret escaped")
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestLibraryRequiresPlatformAdminAndRejectsUnknownFields(t *testing.T) {
	m, _ := settings.New(config.AIConf{Provider: "mock", APIKey: "hidden-key"}, "", "")
	s := &svc.ServiceContext{Settings: m}
	for _, role := range []string{"customer", "platform_service", "shop_admin", "platform_super"} {
		r := libraryRequest("GET", "")
		r.Header.Set("X-User-Roles", role)
		w := httptest.NewRecorder()
		modelLibraryHandler(s, "list")(w, r)
		if strings.Contains(w.Body.String(), `"code":0`) != (role == "platform_super") {
			t.Fatal(role, w.Body.String())
		}
		if strings.Contains(w.Body.String(), "hidden-key") || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("unsafe response")
		}
	}
	w := httptest.NewRecorder()
	modelLibraryHandler(s, "uses")(w, libraryRequest("PUT", `{"revision":0,"uses":{},"apiKey":"discard"}`))
	if strings.Contains(w.Body.String(), `"code":0`) {
		t.Fatal("unknown fields accepted")
	}
}

func TestModelLibraryControlsOnlyCustomerChoices(t *testing.T) {
	initial := config.AIConf{Provider: "volcengine", BaseURL: "https://ark.cn-beijing.volces.com/api/v3",
		APIKey: "fixture-key", Model: "ep-fixture-text", VisionModel: "ep-fixture-vision"}
	m, err := settings.New(initial, t.TempDir(), base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatal(err)
	}
	s := &svc.ServiceContext{Settings: m}
	put := func(body string, code int) {
		t.Helper()
		out := httptest.NewRecorder()
		modelLibraryHandler(s, "uses")(out, libraryRequest("PUT", body))
		var response struct{ Code int }
		if err := json.Unmarshal(out.Body.Bytes(), &response); err != nil || response.Code != code {
			t.Fatal(out.Body.String(), err)
		}
	}
	library := func(ids []string, revision int64) {
		t.Helper()
		out := httptest.NewRecorder()
		modelLibraryHandler(s, "list")(out, libraryRequest("GET", ""))
		var response struct {
			Data struct {
				CustomerModelIDs []string `json:"customerModelIds"`
				Revision         int64    `json:"revision"`
			}
		}
		if err := json.Unmarshal(out.Body.Bytes(), &response); err != nil || response.Data.CustomerModelIDs == nil ||
			!slices.Equal(response.Data.CustomerModelIDs, ids) || response.Data.Revision != revision {
			t.Fatal(out.Body.String(), err)
		}
	}
	assertModels := func(userType, selected, vision string, count int) {
		t.Helper()
		r := httptest.NewRequest("GET", "/api/v1/ai/models", nil)
		r.Header.Set("X-User-Id", "1")
		r.Header.Set("X-User-Type", userType)
		out := httptest.NewRecorder()
		modelListHandler(s)(out, r)
		var response struct{ Data provider.ModelList }
		if err := json.Unmarshal(out.Body.Bytes(), &response); err != nil ||
			response.Data.DefaultModel != selected || response.Data.DefaultVisionModel != vision || len(response.Data.List) != count {
			t.Fatal(out.Body.String(), err)
		}
	}
	library([]string{}, 0)
	assertModels("user", initial.Model, initial.VisionModel, 2)
	put(`{"uses":{},"customerModelIds":["engine-only"],"revision":0}`, 40001)
	put(`{"uses":{},"customerModelIds":["ep-fixture-vision","ep-fixture-vision"],"revision":0}`, 40001)
	library([]string{}, 0)
	put(`{"uses":{"chat":"ep-fixture-text","vision":"ep-fixture-vision"},"customerModelIds":["ep-fixture-vision"],"revision":0}`, 0)
	library([]string{initial.VisionModel}, 1)
	assertModels("user", initial.VisionModel, initial.VisionModel, 1)
	assertModels("admin", initial.Model, initial.VisionModel, 2)
	put(`{"uses":{},"customerModelIds":[],"revision":0}`, 40901)
	put(`{"uses":{},"revision":1}`, 0)
	library([]string{initial.VisionModel}, 2)
	assertModels("user", initial.VisionModel, initial.VisionModel, 1)
	put(`{"uses":{},"customerModelIds":[],"revision":2}`, 0)
	library([]string{}, 3)
	assertModels("user", initial.Model, initial.VisionModel, 2)
	put(`{"uses":{},"customerModelIds":["ep-fixture-text"],"revision":3}`, 0)
	assertModels("user", initial.Model, "", 1)
}

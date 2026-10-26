package modeling

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/askxuan/common"
	"github.com/askxuan/diy-service/internal/model"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func TestModelingMySQL(t *testing.T) {
	if os.Getenv("DIY_MODELING_TEST_ISOLATED") != "1" {
		t.Skip("requires disposable modeling database")
	}
	dsn := os.Getenv("DIY_MODELING_TEST_DSN")
	if !strings.Contains(dsn, "tcp(127.0.0.1:") || !strings.Contains(dsn, "/askxuan_diy?") {
		t.Fatal("disposable local database required")
	}
	ctx := context.Background()
	db := sqlx.NewMysql(dsn)
	secret := strings.Repeat("s", 40)
	for _, table := range []string{"diy_modeling_task", "diy_modeling_config"} {
		if _, e := db.ExecCtx(ctx, "DELETE FROM "+table); e != nil {
			t.Fatal(e)
		}
	}
	creates, polls, stores := 0, 0, 0
	photo, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVQIHWP4z8DwHwAFgAI/ScLbtAAAAABJRU5ErkJggg==")
	service := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/internal/") {
			c, e := common.ParseToken(secret, strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
			if e != nil || c.UserType != "service" || c.ClientID != "diy-service" {
				t.Error("invalid internal token")
			}
			if r.URL.Path == "/internal/diy-model-photos" {
				w.Header().Set("Content-Type", "image/png")
				_, _ = w.Write(photo)
				return
			}
			stores++
			body, _ := io.ReadAll(r.Body)
			if len(body) == 0 {
				t.Error("empty model upload")
			}
			common.Ok(w, map[string]any{"url": "https://files.example/diy-models/" + strings.TrimPrefix(r.URL.Path, "/internal/diy-models/")})
			return
		}
		if r.Method == http.MethodPost {
			creates++
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if !strings.HasPrefix(body["image_urls"].([]any)[0].(string), "data:image/png;base64,") {
				t.Error("photo not embedded")
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"result": "provider-fixture"})
			return
		}
		polls++
		_ = json.NewEncoder(w).Encode(map[string]any{"status": "SUCCEEDED", "progress": 100, "model_urls": map[string]string{"glb": "https://assets.meshy.ai/model.glb", "usdz": "https://assets.meshy.ai/model.usdz"}})
	}))
	defer service.Close()
	manager := func() *Manager {
		m := New(db, secret, service.URL)
		m.ProviderURL = service.URL
		m.DownloadModel = func(context.Context, string) ([]byte, error) { return []byte("fixture-model"), nil }
		return m
	}
	m := manager()
	request := CreateTask{RequestID: "fixture-request-0001", PhotoURLs: []string{"https://files.example/temp/0123456789abcdef0123456789abcdef.png"}}
	if _, e := m.Create(ctx, 7, request); !errors.Is(e, ErrUnavailable) {
		t.Fatal("missing key must block generation", e)
	}
	cfg, e := m.UpdateConfig(ctx, 7, ConfigUpdate{APIKey: "fixture-private-key", Model: "meshy-7.1"})
	if e != nil || !cfg.Configured {
		t.Fatal(cfg, e)
	}
	var stored string
	_ = db.QueryRowCtx(ctx, &stored, "SELECT api_key_cipher FROM diy_modeling_config WHERE id=1")
	if strings.Contains(stored, "fixture-private-key") {
		t.Fatal("plaintext credential persisted")
	}
	task, e := m.Create(ctx, 7, request)
	if e != nil {
		t.Fatal(e)
	}
	again, e := m.Create(ctx, 7, request)
	if e != nil || task.ID != again.ID {
		t.Fatal("idempotent create failed", e)
	}
	changed := request
	changed.PhotoURLs = []string{"https://files.example/another.png"}
	if _, e = m.Create(ctx, 7, changed); !errors.Is(e, ErrConflict) {
		t.Fatal("changed retry silently reused", e)
	}
	m.Poll(ctx)
	task, e = m.Task(ctx, task.ID, 7, false)
	if e != nil || task.Status != "processing" || creates != 1 {
		t.Fatal("create transition", task, e, creates)
	}
	// Recreate the manager, retaining only durable DB state.
	m = manager()
	_, _ = db.ExecCtx(ctx, "UPDATE diy_modeling_task SET next_poll_at=NOW() WHERE id=?", task.ID)
	m.Poll(ctx)
	done, e := m.Task(ctx, task.ID, 7, false)
	if e != nil || done.Status != "succeeded" || done.Model == nil || stores != 2 || polls != 1 || creates != 1 {
		t.Fatal("restart completion", done, e, stores, polls, creates)
	}
	if _, e = m.Task(ctx, task.ID, 8, false); !errors.Is(e, ErrTask) {
		t.Fatal("cross-operator task leaked")
	}
	if _, e = m.Task(ctx, task.ID, 8, true); e != nil {
		t.Fatal("super admin cannot manage job")
	}
	studio := &model.MaterialStudio{Source: "generated", Model: done.Model}
	if e = m.ValidateModel(ctx, studio, 7, false); e != nil {
		t.Fatal(e)
	}
	copyModel := *done.Model
	copyModel.GLBURL = "https://evil.example/model.glb"
	studio.Model = &copyModel
	if m.ValidateModel(ctx, studio, 7, false) == nil {
		t.Fatal("forged model URL accepted")
	}
	_, _ = db.ExecCtx(ctx, "UPDATE diy_modeling_task SET status='submitting',updated_at=DATE_SUB(NOW(),INTERVAL 6 MINUTE) WHERE id=?", task.ID)
	m.Poll(ctx)
	unknown, _ := m.Task(ctx, task.ID, 7, false)
	if unknown.Status != "failed" || creates != 1 {
		t.Fatal("uncertain billed request retried")
	}
	// Ordinary studio materials are drafts; publication and edits preserve legacy behavior.
	mats := model.NewMaterialModel(db)
	s := model.MaterialStudio{Version: 1, Part: "bead", Motion: "fixed", Source: "template", TemplateID: "round", DimensionsMM: []float64{8, 8, 8}, Rotation: []float64{0, 0, 0}, Mount: []float64{0, 0, 0}, RigidFraction: .3}
	asset, _ := json.Marshal(model.RenderAssets{Studio: &s})
	material, e := mats.Insert(ctx, &model.Material{Name: "fixture material", Spec: "8mm", Unit: "颗", UnitPrice: 1, Category: "main_bead", RenderAssets: string(asset), Stock: 2})
	if e != nil || material.Status != "off_shelf" {
		t.Fatal("new studio must draft", material, e)
	}
	if e = mats.UpdateStatus(ctx, material.Id, "on_shelf"); e == nil {
		t.Fatal("unconfirmed draft published")
	}
	s.PreviewConfirmed = true
	asset, _ = json.Marshal(model.RenderAssets{Studio: &s})
	material.RenderAssets = string(asset)
	if e = mats.Update(ctx, material); e != nil {
		t.Fatal(e)
	}
	if e = mats.UpdateStatus(ctx, material.Id, "on_shelf"); e != nil {
		t.Fatal(e)
	}
	s.DimensionsMM = []float64{9, 9, 9}
	asset, _ = json.Marshal(model.RenderAssets{Studio: &s})
	material.RenderAssets = string(asset)
	if e = mats.Update(ctx, material); e != nil {
		t.Fatal(e)
	}
	material, e = mats.FindOne(ctx, material.Id)
	if e != nil || material.Status != "off_shelf" {
		t.Fatal("shape edit remained published", e)
	}
	// A client retaining previewConfirmed cannot change a published appearance
	// without returning to draft. Pricing/stock-only operations stay published.
	for _, change := range []struct {
		name  string
		apply func(*model.Material)
	}{
		{"color", func(v *model.Material) { v.ColorHex = "#102030" }},
		{"texture", func(v *model.Material) { v.TextureKey = "fleck" }},
		{"albedo", func(v *model.Material) {
			asset, _ := json.Marshal(model.RenderAssets{Studio: &s, AlbedoMapURL: "https://files.example/albedo.png"})
			v.RenderAssets = string(asset)
		}},
	} {
		t.Run("appearance_"+change.name, func(t *testing.T) {
			if err := mats.UpdateStatus(ctx, material.Id, "on_shelf"); err != nil {
				t.Fatal(err)
			}
			change.apply(material)
			if !model.StudioAssets(material.RenderAssets).PreviewConfirmed {
				t.Fatal("fixture lost confirmation")
			}
			if err := mats.Update(ctx, material); err != nil {
				t.Fatal(err)
			}
			current, err := mats.FindOne(ctx, material.Id)
			if err != nil || current.Status != "off_shelf" {
				t.Fatal("appearance edit remained published", err)
			}
		})
	}
	if e = mats.UpdateStatus(ctx, material.Id, "on_shelf"); e != nil {
		t.Fatal(e)
	}
	material.UnitPrice, material.Stock = 2, 3
	if e = mats.Update(ctx, material); e != nil {
		t.Fatal(e)
	}
	current, e := mats.FindOne(ctx, material.Id)
	if e != nil || current.Status != "on_shelf" || current.UnitPrice != 2 || current.Stock != 3 {
		t.Fatal("price/stock update changed publication", e)
	}
	cfgJSON, _ := json.Marshal(cfg)
	if bytes.Contains(cfgJSON, []byte("fixture-private-key")) {
		t.Fatal("API returned key")
	}
}

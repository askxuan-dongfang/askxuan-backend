package agentops

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/askxuan/ai-service/internal/config"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/settings"
	mysql "github.com/go-sql-driver/mysql"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type fixtureSkills struct{ items []*model.AISkill }

func (s fixtureSkills) List(context.Context, string) ([]*model.AISkill, error) { return s.items, nil }
func (s fixtureSkills) FindByCode(_ context.Context, code string) (*model.AISkill, error) {
	for _, v := range s.items {
		if v.Code == code {
			return v, nil
		}
	}
	return nil, errors.New("missing")
}
func catalog() fixtureSkills {
	return fixtureSkills{[]*model.AISkill{{Code: "general", Name: "日常问事", Version: "1.0.0", Status: "enabled", PromptTemplate: "fixture-original", InputSchema: `{"fields":[]}`, ToolConfig: `{"enabled":false}`}}}
}
func TestFreezeBindsReviewedContracts(t *testing.T) {
	skills := catalog()
	c := Default(skills.items)
	c.Skills[0].UseTool = true
	if _, e := Freeze(c, skills.items); e == nil {
		t.Fatal("unconfigured tool enabled")
	}
	c.Skills[0].UseTool = false
	c.Skills[0].Code = "shell"
	if _, e := Freeze(c, skills.items); e == nil {
		t.Fatal("unregistered skill accepted")
	}
	c = Default(skills.items)
	f, e := Freeze(c, skills.items)
	if e != nil {
		t.Fatal(e)
	}
	skills.items[0].PromptTemplate = "changed outside release"
	if f.Skills[0].PromptTemplate != "fixture-original" {
		t.Fatal("release references mutable source")
	}
	view := &SkillView{Frozen: f, Version: 7}
	v, e := view.FindByCode(context.Background(), "general")
	if e != nil {
		t.Fatal(e)
	}
	v.PromptTemplate = "mutated by caller"
	again, _ := view.FindByCode(context.Background(), "general")
	if again.PromptTemplate != "fixture-original" || !strings.Contains(again.Version, "@a7") {
		t.Fatal("request snapshot changed")
	}
	c.Skills[0].Enabled = false
	if _, e := Freeze(c, skills.items); e == nil {
		t.Fatal("empty capability set accepted")
	}
}

func TestOperationsMySQLLifecycle(t *testing.T) {
	dsn := os.Getenv("AI_AGENT_OPS_TEST_DSN")
	if dsn == "" {
		t.Skip("isolated AI_AGENT_OPS_TEST_DSN not set")
	}
	parsed, e := mysql.ParseDSN(dsn)
	if e != nil || parsed.DBName != "askxuan_agentops_test" || !strings.HasPrefix(parsed.Addr, "127.0.0.1:") {
		t.Fatal("requires dedicated loopback askxuan_agentops_test database")
	}
	db := sqlx.NewMysql(dsn)
	sqlx.DisableLog()
	ctx := context.Background()
	migration, e := os.ReadFile("../../../../../scripts/db/20260928_ai_agent_operations.sql")
	if e != nil {
		t.Fatal(e)
	}
	for _, stmt := range strings.Split(string(migration), ";") {
		if strings.TrimSpace(stmt) != "" {
			if _, e = db.ExecCtx(ctx, stmt); e != nil {
				t.Fatal(e)
			}
		}
	}
	for _, table := range []string{"ai_agent_debug", "ai_agent_version", "ai_agent_audit"} {
		if _, e = db.ExecCtx(ctx, "DELETE FROM "+table); e != nil {
			t.Fatal(e)
		}
	}
	if _, e = db.ExecCtx(ctx, "UPDATE ai_agent_workspace SET revision=0,draft_json=NULL,active_version=0 WHERE id=1"); e != nil {
		t.Fatal(e)
	}
	repo := &SQLRepository{DB: db}
	sm, e := settings.New(config.AIConf{Provider: "mock", MaxInputChars: 2000, MaxOutputTokens: 1024}, "", "")
	if e != nil {
		t.Fatal(e)
	}
	skills := catalog()
	m := New(repo, skills, sm.VersionedSnapshot, nil, true)
	w, e := m.Workspace(ctx)
	if e != nil || w.Tested || w.ActiveVersion != 0 {
		t.Fatalf("initial workspace: %+v %v", w, e)
	}
	if _, e = m.Publish(ctx, 0, "1", "without test"); !errors.Is(e, ErrUntested) {
		t.Fatal("untested draft published", e)
	}
	var success, conflict atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := m.Save(ctx, 0, w.Draft, "1")
			if err == nil {
				success.Add(1)
			} else if errors.Is(err, ErrConflict) {
				conflict.Add(1)
			} else {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if success.Load() != 1 || conflict.Load() != 1 {
		t.Fatal("concurrent editor overwrite")
	}
	s, e := repo.State(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = repo.Publish(ctx, s.Revision, 0, "1", "unverified"); !errors.Is(e, ErrUntested) {
		t.Fatal("repository bypassed test gate", e)
	}
	d, e := m.StartDebug(ctx, "1", DebugRequest{Revision: s.Revision, Kind: "classic", SkillCode: "general", Question: "合成测试问题", Inputs: map[string]any{}})
	if e != nil {
		t.Fatal(e)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		d, e = m.Debug(ctx, d.ID)
		if e != nil {
			t.Fatal(e)
		}
		if d.Status != "running" {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if d.Status != "completed" || d.PromptTokens == nil || len(d.Events) < 3 {
		t.Fatalf("debug did not finish: %+v", d)
	}
	if _, e = repo.Publish(ctx, s.Revision, 1, "1", "provider changed"); !errors.Is(e, ErrUntested) {
		t.Fatal("test from another provider revision accepted", e)
	}
	var versions [2]int64
	success.Store(0)
	conflict.Store(0)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			v, err := m.Publish(ctx, s.Revision, "1", "verified version")
			versions[i] = v
			if err == nil {
				success.Add(1)
			} else if errors.Is(err, ErrConflict) {
				conflict.Add(1)
			} else {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	if success.Load() != 1 || conflict.Load() != 1 {
		t.Fatal("duplicate concurrent publish")
	}
	active, version, e := m.Active(ctx)
	if e != nil || active == nil || version <= 0 {
		t.Fatal("published version not active", e)
	}
	savedVersion := version
	s, _ = repo.State(ctx)
	c := active.Config
	c.Instruction = "changed draft"
	if e = m.Save(ctx, s.Revision, c, "1"); e != nil {
		t.Fatal(e)
	}
	active2, version2, e := m.Active(ctx)
	if e != nil || version2 != savedVersion || active2.Config.Instruction == "changed draft" {
		t.Fatal("draft leaked to live runtime")
	}
	s, _ = repo.State(ctx)
	if _, e = m.Publish(ctx, s.Revision, "1", "stale test"); !errors.Is(e, ErrUntested) {
		t.Fatal("old test reused for changed draft", e)
	}
	if e = m.Rollback(ctx, s.Revision, 0, "1", "fallback"); e != nil {
		t.Fatal(e)
	}
	f, _, e := m.Active(ctx)
	if e != nil || f != nil {
		t.Fatal("fallback not restored")
	}
	s, _ = repo.State(ctx)
	if e = m.Rollback(ctx, s.Revision, savedVersion, "1", "restore known version"); e != nil {
		t.Fatal(e)
	}
	w, e = m.Workspace(ctx)
	if e != nil || len(w.Versions) != 1 || len(w.Audit) != 5 {
		t.Fatalf("audit/version integrity: %+v %v", w, e)
	}
	// Pending records survive restarts but must not promise resumability without
	// the in-memory Eino checkpoint. They are explicitly expired on retrieval.
	pending := DebugRun{ID: "restart-case", Actor: "1", Kind: "eino", Status: "awaiting_input", InterruptID: "owned-id", StartedAt: time.Now().Format(time.RFC3339)}
	if e = repo.PutDebug(ctx, pending); e != nil {
		t.Fatal(e)
	}
	expired, e := m.Debug(ctx, pending.ID)
	if e != nil || expired.Status != "expired" || expired.InterruptID != "" {
		t.Fatal("lost checkpoint presented as resumable")
	}
	pending.ID = "owner-case"
	pending.Status = "awaiting_input"
	if e = repo.PutDebug(ctx, pending); e != nil {
		t.Fatal(e)
	}
	m.mu.Lock()
	m.tasks[pending.ID] = &debugTask{created: time.Now(), run: pending}
	m.mu.Unlock()
	if e = m.ResumeDebug(ctx, "2", pending.ID, "owned-id", map[string]any{}); !errors.Is(e, ErrConflict) {
		t.Fatal("other actor resumed checkpoint")
	}
	if e = m.CancelDebug(ctx, "2", pending.ID); !errors.Is(e, ErrConflict) {
		t.Fatal("other actor cancelled checkpoint")
	}
	if e = m.CancelDebug(ctx, "1", pending.ID); e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(w)
	if strings.Contains(string(raw), "definition_json") || strings.Contains(string(raw), "apiKey") {
		t.Fatal("private persistence data exposed")
	}
}

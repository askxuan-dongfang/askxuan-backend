package agentops

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/askxuan/ai-service/internal/config"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/askxuan/ai-service/internal/settings"
)

type checkpointMCP struct{ calls atomic.Int32 }

func (m *checkpointMCP) Call(context.Context, string, string) (string, error) {
	m.calls.Add(1)
	return `{"evidence":"synthetic-calculation"}`, nil
}

func checkpointLifecycle(t *testing.T, repo *SQLRepository) {
	t.Helper()
	ctx := context.Background()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			fmt.Fprint(w, `{"data":[{"id":"fixture"}]}`)
			return
		}
		var req struct {
			Messages []struct {
				Role string `json:"role"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		hasTool := false
		for _, msg := range req.Messages {
			if msg.Role == "tool" {
				hasTool = true
			}
		}
		delta := map[string]any{"role": "assistant", "content": "恢复成功，以下仅为合成资料的参考解读，不代表真实预测。"}
		reason := "stop"
		if !hasTool {
			delta = map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"index": 0, "id": "checkpoint-call", "type": "function", "function": map[string]any{"name": "calculate_bazi", "arguments": "{}"}}}}
			reason = "tool_calls"
		}
		w.Header().Set("Content-Type", "text/event-stream")
		b, _ := json.Marshal(map[string]any{"id": "checkpoint-test", "object": "chat.completion.chunk", "model": "fixture", "choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": reason}}})
		fmt.Fprintf(w, "data: %s\n\ndata: [DONE]\n\n", b)
	}))
	defer upstream.Close()
	p := provider.NewOpenAICompatible(upstream.URL+"/v1", "test-only", "fixture", "")
	p.UseStandardParameters()
	p.SetHTTPClient(upstream.Client())
	snap := &settings.Snapshot{Provider: p, Models: provider.NewCatalog(p), Config: config.AIConf{MaxInputChars: 2000, MaxOutputTokens: 1024}}
	var revision atomic.Int64
	snapshot := func() (*settings.Snapshot, int64) { return snap, revision.Load() }
	skills := catalog()
	skills.items = append(skills.items, &model.AISkill{Code: "bazi", Status: "enabled", Version: "1", InputSchema: `{"fields":[{"key":"birthDate","type":"date","required":true},{"key":"birthTime","type":"time","required":true},{"key":"gender","type":"select","required":true,"options":[{"value":"male"},{"value":"female"}]}]}`, ToolConfig: `{"enabled":true,"server":"fixture","tool":"bazi"}`})
	key := base64.StdEncoding.EncodeToString(make([]byte, 32))
	mcp := &checkpointMCP{}
	makeManager := func() *Manager {
		m := New(repo, skills, snapshot, mcp, true)
		if err := m.EnablePersistence(key); err != nil {
			t.Fatal(err)
		}
		return m
	}
	first := makeManager()
	state, _ := repo.State(ctx)
	cfg := Default(skills.items)
	cfg.Model = "fixture"
	if err := first.Save(ctx, state.Revision, cfg, "1"); err != nil {
		t.Fatal(err)
	}
	state, _ = repo.State(ctx)
	run, err := first.StartDebug(ctx, "1", DebugRequest{Revision: state.Revision, Kind: "eino", SkillCode: "bazi", Question: "仅用于合成测试的私人问题", Inputs: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	wait := func(m *Manager, id, status string) DebugRun {
		t.Helper()
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			d, e := m.Debug(ctx, id)
			if e != nil {
				t.Fatal(e)
			}
			if d.Status == status {
				return d
			}
			if d.Status == "failed" || d.Status == "expired" {
				t.Fatalf("unexpected terminal state %+v", d)
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("debug timed out")
		return DebugRun{}
	}
	paused := wait(first, run.ID, "awaiting_input")
	row, err := first.checkpoints.row(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(row.Ciphertext, "私人问题") || strings.Contains(row.Ciphertext, "birthDate") {
		t.Fatal("plaintext checkpoint")
	}
	if _, err = first.checkpoints.open("other-task", row.Ciphertext); err == nil {
		t.Fatal("ciphertext not bound to task")
	}
	damaged := []byte(row.Ciphertext)
	damaged[len(damaged)/2] = '!'
	if _, err = first.checkpoints.open(run.ID, string(damaged)); err == nil {
		t.Fatal("tampered ciphertext accepted")
	}
	// New managers have no in-memory task or harness from the original instance.
	second, third := makeManager(), makeManager()
	inputs := map[string]any{"birthDate": "1990-01-02", "birthTime": "12:30", "gender": "male"}
	if err = second.ResumeDebug(ctx, "2", run.ID, paused.InterruptID, inputs); err == nil {
		t.Fatal("wrong actor resumed")
	}
	revision.Store(1)
	if err = second.ResumeDebug(ctx, "1", run.ID, paused.InterruptID, inputs); err == nil {
		t.Fatal("changed provider accepted")
	}
	revision.Store(0)
	var wins atomic.Int32
	var wg sync.WaitGroup
	for _, m := range []*Manager{second, third} {
		wg.Add(1)
		go func(m *Manager) {
			defer wg.Done()
			if m.ResumeDebug(ctx, "1", run.ID, paused.InterruptID, inputs) == nil {
				wins.Add(1)
			}
		}(m)
	}
	wg.Wait()
	done := wait(second, run.ID, "completed")
	if wins.Load() != 1 || mcp.calls.Load() != 1 || done.ModelAttempts < paused.ModelAttempts || done.ToolAttempts < paused.ToolAttempts {
		t.Fatalf("duplicate execution/budget reset: winners=%d calls=%d %+v", wins.Load(), mcp.calls.Load(), done)
	}
	// Completion eventually deletes the encrypted checkpoint, even if a final
	// event becomes visible just before the cleanup transaction commits.
	deadline := time.Now().Add(time.Second)
	var n int
	for time.Now().Before(deadline) {
		_ = repo.DB.QueryRowCtx(ctx, &n, `SELECT COUNT(*) FROM ai_agent_checkpoint WHERE id=?`, run.ID)
		if n == 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if n != 0 {
		t.Fatal("completed private checkpoint retained")
	}
}

func TestRolloutBucketsStableAcrossIncreases(t *testing.T) {
	low := Rollout{VersionID: 2, StableVersion: 1, Percentage: 10}
	high := low
	high.Percentage = 50
	count := 0
	for i := 0; i < 1000; i++ {
		subject := fmt.Sprint(i)
		if low.Select(subject) == 2 {
			count++
			if high.Select(subject) != 2 {
				t.Fatal("user left cohort after increase")
			}
		}
	}
	if count < 50 || count > 150 || low.Select("") != 1 {
		t.Fatal("invalid cohort distribution", count)
	}
	high.Percentage = 100
	if high.Select("") != 2 {
		t.Fatal("full rollout ignored")
	}
}

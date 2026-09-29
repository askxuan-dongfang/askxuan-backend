package svc

import (
	"context"
	"github.com/askxuan/ai-service/internal/agent"
	"github.com/askxuan/ai-service/internal/config"
	"github.com/askxuan/ai-service/internal/settings"
	"strings"
	"testing"
)

func TestRuntimeUsesSnapshotInputGuardAndLeavesOldRequestAlone(t *testing.T) {
	manager, e := settings.New(config.AIConf{Provider: "mock", MaxInputChars: 20000, MaxOutputTokens: 8192}, "", "")
	if e != nil {
		t.Fatal(e)
	}
	old := &ServiceContext{Settings: manager, Guard: agent.NewGuard(2000, nil)}
	next := old.Runtime()
	long := strings.Repeat("字", 5000)
	if _, e = next.Guard.Validate(`{"fields":[]}`, long, nil); e != nil {
		t.Fatal(e)
	}
	if _, e = old.Guard.Validate(`{"fields":[]}`, long, nil); e == nil {
		t.Fatal("mutated old guard")
	}
	if _, e = next.AskRuntime(context.Background()); e != nil {
		t.Fatal(e)
	}
}

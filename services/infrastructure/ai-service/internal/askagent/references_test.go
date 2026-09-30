package askagent

import (
	"context"
	"strings"
	"testing"
)

func TestReferenceToolCannotChooseAnotherUser(t *testing.T) {
	calls := 0
	r := referenceTool{name: "recall_memory", read: func(context.Context, string, string) (string, error) { calls++; return "owned", nil }}
	for _, args := range []string{`{"userId":"other"}`, `null`, `[]`} {
		if v, e := r.InvokableRun(context.Background(), args); e != nil || !strings.Contains(v, `"ok":false`) {
			t.Fatal("unscoped args accepted")
		}
	}
	if calls != 0 {
		t.Fatal(calls)
	}
	v, e := r.InvokableRun(context.Background(), `{}`)
	if e != nil || v != "owned" || calls != 1 {
		t.Fatal(v, e)
	}
}

func TestKnowledgeQueryCannotWidenScope(t *testing.T) {
	calls := 0
	r := referenceTool{name: "search_knowledge", read: func(_ context.Context, name, query string) (string, error) {
		calls++
		if name != "search_knowledge" || query != "周易 乾 九二" {
			t.Fatalf("wrong query %q", query)
		}
		return "scoped", nil
	}}
	for _, args := range []string{`{"query":"x","userId":"other"}`, `{"knowledgeBaseIds":["other"]}`, `{"query":123}`, `{"query":" "}`, `{"query":"` + strings.Repeat("字", 401) + `"}`, `null`, `[]`} {
		value, err := r.InvokableRun(context.Background(), args)
		if err != nil || !strings.Contains(value, `"ok":false`) {
			t.Fatalf("invalid accepted: %s", args)
		}
	}
	if calls != 0 {
		t.Fatal("invalid query executed")
	}
	value, err := r.InvokableRun(context.Background(), `{"query":"  周易 乾 九二  "}`)
	if err != nil || value != "scoped" || calls != 1 {
		t.Fatal(value, err, calls)
	}
	r.name = "recall_memory"
	if value, _ = r.InvokableRun(context.Background(), `{"query":"another user"}`); !strings.Contains(value, `"ok":false`) || calls != 1 {
		t.Fatal("memory scope changed")
	}
}

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

func TestWebSearchRequiresExplicitQuery(t *testing.T) {
	calls := 0
	tool := &referenceTool{name: "search_web", read: func(_ context.Context, name, q string) (string, error) {
		calls++
		if name != "search_web" || q != "古籍" {
			t.Fatal("unexpected query")
		}
		return "ok", nil
	}}
	for _, a := range []string{`{}`, `{"query":""}`, `{"query":"古籍","owner":"another"}`, `null`} {
		out, e := tool.InvokableRun(context.Background(), a)
		if e != nil || !strings.Contains(out, "invalid_model_arguments") {
			t.Fatal("invalid input accepted")
		}
	}
	if calls != 0 {
		t.Fatal("search happened without valid arguments")
	}
	if _, e := tool.InvokableRun(context.Background(), `{"query":"古籍"}`); e != nil || calls != 1 {
		t.Fatal("valid query failed")
	}
}

func TestWebPageReaderStrictArguments(t *testing.T) {
	calls := 0
	tool := &referenceTool{name: "read_webpage", read: func(_ context.Context, name, url string) (string, error) {
		calls++
		if name != "read_webpage" || url != "https://example.org/book" {
			t.Fatal("wrong reader arguments")
		}
		return "page", nil
	}}
	for _, args := range []string{`{}`, `null`, `{"query":"anything"}`, `{"url":123}`, `{"url":"https://example.org/book","headers":{"Authorization":"secret"}}`} {
		raw, err := tool.InvokableRun(context.Background(), args)
		if err != nil || !strings.Contains(raw, "invalid_url") {
			t.Fatal("invalid reader arguments accepted")
		}
	}
	if calls != 0 {
		t.Fatal("invalid read executed")
	}
	raw, err := tool.InvokableRun(context.Background(), `{"url":"https://example.org/book"}`)
	if err != nil || raw != "page" || calls != 1 {
		t.Fatal("reader failed")
	}
}

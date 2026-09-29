package askagent

import (
	"context"
	"testing"
)

func TestReferenceToolCannotChooseAnotherUser(t *testing.T) {
	calls := 0
	r := referenceTool{name: "recall_memory", read: func(context.Context, string) (string, error) { calls++; return "owned", nil }}
	for _, args := range []string{`{"userId":"other"}`, `null`, `[]`} {
		if _, e := r.InvokableRun(context.Background(), args); e == nil {
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

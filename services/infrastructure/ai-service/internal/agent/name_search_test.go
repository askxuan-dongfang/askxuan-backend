package agent

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNameSearchProviderRequiresMatchingProvenance(t *testing.T) {
	valid := false
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test" {
			t.Error("missing auth")
		}
		if valid {
			fmt.Fprint(w, `{"name":"李明","source":"授权测试库","scope":"测试地区","asOf":"2026-09-30","result":"测试匹配1条"}`)
		} else {
			fmt.Fprint(w, `{"name":"另一个人","result":"0"}`)
		}
	}))
	defer s.Close()
	t.Setenv("AI_NAME_DUPLICATE_URL", s.URL)
	t.Setenv("AI_NAME_DUPLICATE_KEY", "test")
	in := map[string]any{"nameSearch": "duplicate", "name": "李明", "surname": "李"}
	out := nameSearch(context.Background(), in)
	if len(out) != 1 || len(out[0].Rows) == 0 {
		t.Fatalf("%+v", out)
	}
	if out[0].Rows[0][1] != "结果未通过核验" {
		t.Fatal(out)
	}
	valid = true
	out = nameSearch(context.Background(), in)
	if out[0].Rows[0][1] != "已查询" {
		t.Fatal(out)
	}
}

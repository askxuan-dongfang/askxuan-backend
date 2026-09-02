package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMCPClientHonorsAllowlistAndParsesResult(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.Header.Get("Mcp-Name") != "bazi" {
			t.Fatalf("unexpected tool header: %s", r.Header.Get("Mcp-Name"))
		}
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":"1","result":{"content":[{"type":"text","text":"排盘结果"}],"structuredContent":{"ok":true}}}`))
	}))
	defer server.Close()
	client := NewMCPClient(true, server.URL, 2)
	result, err := client.Call(context.Background(), `{"enabled":true,"server":"taibu","tool":"bazi"}`, `{"birthDate":"2000-01-01"}`)
	if err != nil || !called || result == "" {
		t.Fatalf("unexpected MCP result: %q %v called=%v", result, err, called)
	}
}

func TestMCPClientDisabledDoesNotCall(t *testing.T) {
	client := NewMCPClient(false, "http://invalid", 1)
	result, err := client.Call(context.Background(), `{"enabled":true,"tool":"bazi"}`, `{}`)
	if err != nil || result != "" {
		t.Fatalf("disabled MCP should be a no-op: %q %v", result, err)
	}
}

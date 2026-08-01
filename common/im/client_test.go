package im

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendMessageMatchesOpenIMV383Contract(t *testing.T) {
	var called bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.URL.Path != "/msg/send_msg" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.Header.Get("token") != "admin-token" {
			t.Fatalf("token header = %q", r.Header.Get("token"))
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["senderNickname"] != "预约用户" || body["senderName"] != nil {
			t.Fatalf("sender nickname payload = %#v", body)
		}
		if body["senderPlatformID"] != float64(1) || body["ex"] != "askxuan-booking:B1:c1" {
			t.Fatalf("platform/marker payload = %#v", body)
		}
		content, ok := body["content"].(map[string]any)
		if !ok || content["content"] != "hello" {
			t.Fatalf("content payload = %#v", body["content"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"errCode": 0})
	}))
	defer server.Close()

	client := NewClient(server.URL, "imAdmin", "secret")
	client.adminToken = "admin-token"
	err := client.SendMessage(context.Background(), &SendMsgReq{
		SendID: "u_1", RecvID: "m_1", SenderName: "预约用户", SenderPlatformID: 1,
		SessionType: 1, ContentType: 101, Content: map[string]string{"content": "hello"},
		Ex: "askxuan-booking:B1:c1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("OpenIM endpoint was not called")
	}
}

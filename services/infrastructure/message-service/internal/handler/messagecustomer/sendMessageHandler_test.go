package messagecustomer

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/askxuan/common"
)

func TestSendMessageHandlerReturnsBusinessErrorJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages/send",
		strings.NewReader(`{"conversationId":"M001","userId":"1","content":"blocked"}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	SendMessageHandler(nil).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var body common.Body
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("response is not JSON: %v; body = %s", err, recorder.Body.String())
	}
	if body.Code != common.ErrBookingChatUnavailable.Code {
		t.Fatalf("code = %d, want %d", body.Code, common.ErrBookingChatUnavailable.Code)
	}
}

package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLegacyOpenIMWebhookIsNoOp(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/openim/webhook/callbackAfterSendSingleMsgCommand",
		strings.NewReader(`{"sendID":"u_attacker","recvID":"m_1","content":"forged","sessionType":1,"contentType":101}`))
	req.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	OpenIMWebhookHandler(nil).ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"code":0`) {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

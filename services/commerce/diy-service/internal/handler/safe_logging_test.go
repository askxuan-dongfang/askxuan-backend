package handler

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/zeromicro/go-zero/core/logx"
)

func TestModelingConfigNeverLogsCredentialsOnFailure(t *testing.T) {
	previous := logx.Reset()
	defer logx.SetWriter(previous)
	for _, verbose := range []bool{false, true} {
		var logs bytes.Buffer
		logx.SetWriter(logx.NewWriter(&logs))
		r := httptest.NewRequest("PUT", "/api/v1/admin/diy/modeling/config?private-key-in-query", strings.NewReader(`{"apiKey":"private-key-in-body"}`))
		r.Header.Set("Authorization", "Bearer private-key-in-header")
		called := false
		SafeModelingLog(verbose)(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			called = strings.Contains(string(body), "private-key-in-body")
			w.WriteHeader(http.StatusInternalServerError)
		})(httptest.NewRecorder(), r)
		if !called || strings.Contains(logs.String(), "private-key") || !strings.Contains(logs.String(), "/modeling/config") {
			t.Fatal("credentials logged or request body altered")
		}
	}
}

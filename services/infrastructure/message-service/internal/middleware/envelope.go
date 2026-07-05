package middleware

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/askxuan/common"
)

// EnvelopeFunc wraps legacy message-service naked JSON responses into
// the platform-wide {code,message,data} contract. Already wrapped responses
// pass through unchanged.
func EnvelopeFunc(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		bw := &bodyWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			body:           bytes.NewBuffer(nil),
		}
		next(bw, r)

		body := bytes.TrimSpace(bw.body.Bytes())
		if len(body) == 0 {
			w.WriteHeader(bw.statusCode)
			return
		}
		contentType := w.Header().Get("Content-Type")
		if contentType != "" && !strings.Contains(contentType, "application/json") {
			w.WriteHeader(bw.statusCode)
			_, _ = w.Write(body)
			return
		}

		var obj map[string]json.RawMessage
		if err := json.Unmarshal(body, &obj); err == nil {
			if _, hasCode := obj["code"]; hasCode {
				if _, hasMessage := obj["message"]; hasMessage {
					w.WriteHeader(bw.statusCode)
					_, _ = w.Write(body)
					return
				}
			}
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		resp, _ := json.Marshal(common.Body{
			Code:    0,
			Message: "success",
			Data:    json.RawMessage(body),
		})
		_, _ = w.Write(resp)
	}
}

type bodyWriter struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (w *bodyWriter) WriteHeader(code int) {
	w.statusCode = code
}

func (w *bodyWriter) Write(p []byte) (int, error) {
	return w.body.Write(p)
}

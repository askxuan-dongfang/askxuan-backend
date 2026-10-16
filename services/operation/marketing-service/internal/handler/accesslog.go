package handler

import (
	"github.com/zeromicro/go-zero/core/logx"
	"net/http"
	"time"
)

// Keep access/error visibility without request/response body dumps: stock
// imports and redemption responses contain bearer credentials.
func AccessLog(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		started := time.Now()
		tracked := &accessWriter{ResponseWriter: w, status: http.StatusOK}
		next(tracked, r)
		logx.WithContext(r.Context()).Infof("[HTTP] %s %s status=%d duration=%s", r.Method, r.URL.Path, tracked.status, time.Since(started))
	}
}

type accessWriter struct {
	http.ResponseWriter
	status int
}

func (w *accessWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

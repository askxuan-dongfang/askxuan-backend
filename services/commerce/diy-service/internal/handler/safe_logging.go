package handler

import (
	"net/http"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
	resthandler "github.com/zeromicro/go-zero/rest/handler"
)

// The framework's default 5xx logger dumps request bodies. Provider credentials
// must remain excluded even on timeout or failure, before the handler can redact.
func SafeModelingLog(verbose bool) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		logger := resthandler.LogHandler(next)
		if verbose {
			logger = resthandler.DetailedLogHandler(next)
		}
		return func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/api/v1/admin/diy/modeling/config" {
				logger.ServeHTTP(w, r)
				return
			}
			started := time.Now()
			defer func() {
				logx.WithContext(r.Context()).WithDuration(time.Since(started)).Infof("[HTTP] %s /api/v1/admin/diy/modeling/config", r.Method)
			}()
			next(w, r)
		}
	}
}

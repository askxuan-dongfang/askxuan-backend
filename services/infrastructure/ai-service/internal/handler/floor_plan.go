package handler

import (
	"encoding/json"
	"github.com/askxuan/ai-service/internal/logic"
	"github.com/askxuan/ai-service/internal/svc"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/rest"
	"io"
	"net/http"
	"sync"
	"time"
)

var floorLimiter = struct {
	sync.Mutex
	last map[string]time.Time
}{last: map[string]time.Time{}}

func registerFloorPlan(server *rest.Server, s *svc.ServiceContext) {
	server.AddRoute(rest.Route{Method: "POST", Path: "/api/v1/ai/floor-plan/parse", Handler: func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		user, err := resolveUserID(r, "")
		if err != nil {
			common.JsonError(w, err)
			return
		}
		var req struct {
			URL    string `json:"url"`
			Height int    `json:"height"`
		}
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		d.DisallowUnknownFields()
		if d.Decode(&req) != nil || req.URL == "" {
			common.JsonError(w, common.ErrParam)
			return
		}
		var extra any
		if d.Decode(&extra) != io.EOF {
			common.JsonError(w, common.ErrParam)
			return
		}
		floorLimiter.Lock()
		now := time.Now()
		for k, t := range floorLimiter.last {
			if now.Sub(t) > time.Minute {
				delete(floorLimiter.last, k)
			}
		}
		_, recent := floorLimiter.last[user]
		if recent || len(floorLimiter.last) >= 100 {
			floorLimiter.Unlock()
			common.JsonError(w, common.NewBizError(42901, "识别较频繁，请一分钟后再试"))
			return
		}
		floorLimiter.last[user] = now
		floorLimiter.Unlock()
		out, err := logic.ParseFloorPlan(r.Context(), s, user, req.URL, req.Height)
		if err != nil {
			common.JsonError(w, common.NewBizError(50301, "户型识别暂不可用，可手动标注并校准；请确认已配置视觉模型"))
			return
		}
		common.Ok(w, out)
	}})
}

package handler

import (
	"encoding/json"
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/common/scheduling"
	"github.com/askxuan/master-service/internal/logic"
	"github.com/askxuan/master-service/internal/model"
	"github.com/askxuan/master-service/internal/svc"
	"github.com/askxuan/master-service/internal/types"
	"github.com/zeromicro/go-zero/rest"
	"net/http"
)

func registerTempleSchedules(server *rest.Server, s *svc.ServiceContext) {
	auth := &middleware.AuthConfig{Secret: s.Config.AuthSecret}
	for _, method := range []string{"GET", "PUT"} {
		method := method
		server.AddRoute(rest.Route{Method: method, Path: "/api/v1/admin/temples/schedules", Handler: auth.AuthFunc(func(w http.ResponseWriter, r *http.Request) {
			claims := middleware.AdminClaimsFromCtx(r.Context())
			if claims == nil || claims.UserType != "admin" || !claims.HasTempleRole() || claims.TempleCode == "" {
				common.JsonError(w, common.ErrForbidden)
				return
			}
			master, date := r.URL.Query().Get("masterCode"), r.URL.Query().Get("date")
			var req struct {
				MasterCode string   `json:"masterCode"`
				Date       string   `json:"date"`
				TimeSlots  []string `json:"timeSlots"`
				Status     string   `json:"status"`
				Revision   *int64   `json:"revision"`
			}
			if method == "PUT" {
				if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req) != nil || req.Revision == nil {
					common.JsonError(w, common.ErrParam)
					return
				}
				master, date = req.MasterCode, req.Date
			}
			if master == "" || !scheduling.ValidDate(date) {
				common.JsonError(w, common.ErrParam)
				return
			}
			m, e := s.MasterModel.FindByCode(r.Context(), master)
			if e != nil || m.TempleCode != claims.TempleCode || m.ManageBy != "temple" {
				common.JsonError(w, common.ErrForbidden)
				return
			}
			if method == "GET" {
				rows, _, e := s.MasterScheduleModel.FindByMasterId(r.Context(), master, date, 1, 1)
				if e != nil {
					common.JsonError(w, common.ErrSystem)
					return
				}
				list := []types.MasterSchedule{}
				for _, row := range rows {
					list = append(list, logic.ScheduleResponse(row))
				}
				common.Ok(w, map[string]any{"list": list})
				return
			}
			b, _ := json.Marshal(req.TimeSlots)
			id, e := s.MasterScheduleModel.Upsert(r.Context(), &model.MasterSchedule{MasterCode: master, ScheduleDate: date, TimeSlots: string(b), Status: req.Status}, req.Revision)
			if e != nil {
				if _, ok := e.(*common.BizError); ok {
					common.JsonError(w, e)
				} else {
					common.JsonError(w, common.ErrSystem)
				}
				return
			}
			common.Ok(w, map[string]any{"id": id})
		})})
	}
}

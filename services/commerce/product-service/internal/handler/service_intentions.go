package handler

import (
	"context"
	"github.com/askxuan/common"
	"github.com/askxuan/product-service/internal/svc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/rest/httpx"
	"net/http"
	"strings"
)

// Gateway strips all external identity headers and verifies admin JWTs. Product
// service is internal-only, matching the existing taxonomy administration API.
func serviceIntentions(s *svc.ServiceContext, write bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-User-Type") != "admin" || !strings.Contains(","+r.Header.Get("X-User-Roles")+",", ",platform_super,") {
			common.JsonError(w, common.ErrForbidden)
			return
		}
		if write {
			var req struct {
				Kind    string   `json:"kind"`
				ID      string   `json:"id"`
				Service string   `json:"serviceCode"`
				Tags    []string `json:"tags"`
			}
			if httpx.Parse(r, &req) != nil || len(req.Tags) > 7 {
				common.JsonError(w, common.ErrParam)
				return
			}
			allowed := map[string]bool{"peace": true, "wealth": true, "love": true, "career": true, "study": true, "taisui": true, "rite": true}
			for _, t := range req.Tags {
				if !allowed[t] {
					common.JsonError(w, common.ErrParam)
					return
				}
			}
			e := s.DB.TransactCtx(r.Context(), func(ctx context.Context, tx sqlx.Session) error {
				var count int
				switch req.Kind {
				case "master":
					if e := tx.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM askxuan_master.master_service_tag WHERE master_code=? AND service_code=?", req.ID, req.Service); e != nil {
						return e
					}
					if count == 0 {
						return common.ErrParam
					}
					if _, e := tx.ExecCtx(ctx, "DELETE FROM master_service_intention WHERE master_code=? AND service_code=?", req.ID, req.Service); e != nil {
						return e
					}
					seen := map[string]bool{}
					for _, t := range req.Tags {
						if seen[t] {
							continue
						}
						seen[t] = true
						if _, e := tx.ExecCtx(ctx, "INSERT INTO master_service_intention(master_code,service_code,tag_code) VALUES(?,?,?)", req.ID, req.Service, t); e != nil {
							return e
						}
					}
				case "temple":
					if e := tx.QueryRowCtx(ctx, &count, "SELECT COUNT(*) FROM askxuan_temple.temple_service WHERE id=? AND service_code=?", req.ID, req.Service); e != nil {
						return e
					}
					if count == 0 {
						return common.ErrParam
					}
					if _, e := tx.ExecCtx(ctx, "DELETE FROM askxuan_temple.temple_service_intent_tag WHERE temple_service_id=?", req.ID); e != nil {
						return e
					}
					seen := map[string]bool{}
					for _, t := range req.Tags {
						if seen[t] {
							continue
						}
						seen[t] = true
						if _, e := tx.ExecCtx(ctx, "INSERT INTO askxuan_temple.temple_service_intent_tag(temple_service_id,tag_code) VALUES(?,?)", req.ID, t); e != nil {
							return e
						}
					}
				default:
					return common.ErrParam
				}
				return nil
			})
			if e != nil {
				common.JsonError(w, e)
				return
			}
			common.Ok(w, map[string]bool{"updated": true})
			return
		}
		var rows []struct {
			Kind        string `db:"kind" json:"kind"`
			ID          string `db:"id" json:"id"`
			Name        string `db:"name" json:"name"`
			Service     string `db:"service_code" json:"serviceCode"`
			ServiceName string `db:"service_name" json:"serviceName"`
			Tags        string `db:"tags" json:"tags"`
		}
		e := s.DB.QueryRowsCtx(r.Context(), &rows, `SELECT 'master' kind,m.code id,m.dharma_name name,ms.service_code,st.name service_name,COALESCE((SELECT GROUP_CONCAT(tag_code ORDER BY tag_code) FROM master_service_intention i WHERE i.master_code=m.code AND i.service_code=ms.service_code),'') tags FROM askxuan_master.master m JOIN askxuan_master.master_service_tag ms ON ms.master_code=m.code JOIN askxuan_temple.service_type st ON st.code=ms.service_code
 UNION ALL SELECT 'temple',CAST(ts.id AS CHAR),t.name,ts.service_code,ts.service_name,COALESCE((SELECT GROUP_CONCAT(tag_code ORDER BY tag_code) FROM askxuan_temple.temple_service_intent_tag i WHERE i.temple_service_id=ts.id),'') FROM askxuan_temple.temple_service ts JOIN askxuan_temple.temple t ON t.code=ts.temple_code ORDER BY kind,id,service_code`)
		if e != nil {
			common.JsonError(w, common.ErrSystem)
			return
		}
		common.Ok(w, map[string]interface{}{"list": rows})
	}
}

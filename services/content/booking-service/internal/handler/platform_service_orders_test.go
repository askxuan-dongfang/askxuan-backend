package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
)

func TestPlatformServiceHTTPRejectsNonSuperBeforeAnyDataAccess(t *testing.T) {
	for _, kind := range []string{"bookings", "booking", "fulfillment", "consultations", "consultation"} {
		for _, identity := range []struct{ userType, role string }{{"admin", "temple_admin"}, {"admin", "master"}, {"admin", "custom_reader"}, {"user", "customer"}, {"service", "platform_super"}} {
			ctx := context.WithValue(context.Background(), middleware.CtxKeyUserType, identity.userType)
			ctx = context.WithValue(ctx, middleware.CtxKeyUserID, int64(9))
			ctx = context.WithValue(ctx, middleware.CtxKeyRoles, []string{identity.role})
			w := httptest.NewRecorder()
			// Nil data dependencies intentionally panic if the handler reads any.
			platformServiceOrdersHandler(&svc.ServiceContext{}, kind)(w, httptest.NewRequest("GET", "/", nil).WithContext(ctx))
			var response struct {
				Code int `json:"code"`
			}
			if json.Unmarshal(w.Body.Bytes(), &response) != nil || response.Code != common.ErrForbidden.Code {
				t.Fatalf("%s %s was not forbidden: %s", identity.role, kind, w.Body.String())
			}
		}
	}
}

func TestPlatformServiceHTTPRejectsMalformedQuery(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.CtxKeyUserType, "admin")
	ctx = context.WithValue(ctx, middleware.CtxKeyUserID, int64(9))
	ctx = context.WithValue(ctx, middleware.CtxKeyRoles, []string{"platform_super"})
	for _, query := range []string{"?page=-1", "?size=101", "?page=not-a-number", "?from=2026-02-30"} {
		w := httptest.NewRecorder()
		platformServiceOrdersHandler(&svc.ServiceContext{}, "bookings")(w, httptest.NewRequest("GET", "/"+query, nil).WithContext(ctx))
		var response struct {
			Code int `json:"code"`
		}
		if json.Unmarshal(w.Body.Bytes(), &response) != nil || response.Code != common.ErrParamInvalid.Code {
			t.Fatalf("malformed query %s accepted: %s", query, w.Body.String())
		}
	}
}

package handler

import (
	"encoding/json"
	"github.com/askxuan/common"
	"github.com/askxuan/payment-service/internal/svc"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPointsRouteAuthorization(t *testing.T) {
	for _, tc := range []struct {
		name, role string
		code       int
	}{
		{"anonymous", "", common.ErrUnauthorized.Code},
		{"customer", "customer", common.ErrRoleForbidden.Code},
		{"temple admin", "temple_admin", common.ErrRoleForbidden.Code},
		{"master", "master", common.ErrRoleForbidden.Code},
		{"shop admin", "shop_admin", common.ErrParam.Code},
		{"platform admin", "platform_super", common.ErrParam.Code},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var c rest.RestConf
			conf.FillDefault(&c)
			c.Name = "points-test"
			c.Port = 33380
			server, err := rest.NewServer(c)
			if err != nil {
				t.Fatal(err)
			}
			defer server.Stop()
			svcCtx := &svc.ServiceContext{}
			svcCtx.Config.Auth.AccessSecret = "points-test-secret"
			registerPoints(server, svcCtx)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/points/products", strings.NewReader(`{"name":""}`))
			req.Header.Set("Content-Type", "application/json")
			if tc.role != "" {
				token, err := common.GenAccessToken("points-test-secret", common.TokenInfo{UserId: 1, Roles: []string{tc.role}}, 3600)
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Set("Authorization", "Bearer "+token)
			}
			rr := httptest.NewRecorder()
			server.ServeHTTP(rr, req)
			var response struct {
				Code int `json:"code"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
				t.Fatalf("%s: %v", rr.Body.String(), err)
			}
			if response.Code != tc.code {
				t.Fatalf("got %d want %d body=%s", response.Code, tc.code, rr.Body.String())
			}
		})
	}
}

func TestCustomerPointsRoleIsolation(t *testing.T) {
	for _, tc := range []struct {
		name, role string
		code       int
	}{
		{"anonymous", "", common.ErrUnauthorized.Code},
		{"customer", "customer", common.ErrParam.Code},
		{"temple admin", "temple_admin", common.ErrRoleForbidden.Code},
		{"master", "master", common.ErrRoleForbidden.Code},
		{"shop admin", "shop_admin", common.ErrRoleForbidden.Code},
		{"platform admin", "platform_super", common.ErrRoleForbidden.Code},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var c rest.RestConf
			conf.FillDefault(&c)
			c.Name = "points-test"
			c.Port = 33380
			server, err := rest.NewServer(c)
			if err != nil {
				t.Fatal(err)
			}
			defer server.Stop()
			svcCtx := &svc.ServiceContext{}
			svcCtx.Config.Auth.AccessSecret = "points-test-secret"
			registerPoints(server, svcCtx)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/points/orders", strings.NewReader(`{"name":""}`))
			req.Header.Set("Content-Type", "application/json")
			if tc.role != "" {
				token, err := common.GenAccessToken("points-test-secret", common.TokenInfo{UserId: 1, Roles: []string{tc.role}}, 3600)
				if err != nil {
					t.Fatal(err)
				}
				req.Header.Set("Authorization", "Bearer "+token)
			}
			rr := httptest.NewRecorder()
			server.ServeHTTP(rr, req)
			var response struct {
				Code int `json:"code"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &response); err != nil {
				t.Fatalf("%s: %v", rr.Body.String(), err)
			}
			if response.Code != tc.code {
				t.Fatalf("got %d want %d body=%s", response.Code, tc.code, rr.Body.String())
			}
		})
	}
}

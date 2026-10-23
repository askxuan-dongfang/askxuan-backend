package handler

import (
	"errors"
	"github.com/askxuan/auth-service/internal/svc"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/rest"
	"net/http/httptest"
	"testing"
)

func TestTempleTeamRoutesAreRemoved(t *testing.T) {
	server := rest.MustNewServer(rest.RestConf{Host: "127.0.0.1", Port: 0})
	defer server.Stop()
	RegisterHandlers(server, &svc.ServiceContext{})
	for _, route := range server.Routes() {
		if route.Path == "/api/v1/auth/temple/team" {
			t.Fatal("retired team API remains registered", route.Method)
		}
	}
}

func TestRetiredTempleDirectAuthRejectsBeforeDependencies(t *testing.T) {
	sc := &svc.ServiceContext{}
	sc.Config.Auth.AccessSecret = "retired-direct-fixture"
	for _, role := range []string{"temple_reception", "temple_operations", "temple_finance"} {
		token, e := common.GenAccessToken(sc.Config.Auth.AccessSecret, common.TokenInfo{UserId: 7, UserType: "admin", Roles: []string{role}}, 60)
		if e != nil {
			t.Fatal(e)
		}
		req := httptest.NewRequest("GET", "/api/v1/auth/access", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		if _, _, e = currentHumanClaims(sc, req); !errors.Is(e, common.ErrSessionRevoked) {
			t.Fatal("retired token reached session dependencies", role, e)
		}
	}
}

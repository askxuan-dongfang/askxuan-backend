package handler

import (
	"encoding/json"
	"github.com/askxuan/common"
	"github.com/askxuan/diy-service/internal/config"
	"github.com/askxuan/diy-service/internal/modeling"
	"github.com/askxuan/diy-service/internal/svc"
	"github.com/zeromicro/go-zero/rest"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestModelingCredentialsRequireHumanSuperAdmin(t *testing.T) {
	secret := strings.Repeat("s", 40)
	s := &svc.ServiceContext{Config: config.Config{AuthSecret: secret}, Modeling: modeling.New(nil, secret, "")}
	server, e := rest.NewServer(rest.RestConf{})
	if e != nil {
		t.Fatal(e)
	}
	registerModeling(server, s)
	var handler rest.Route
	for _, r := range server.Routes() {
		if r.Method == "PUT" && r.Path == "/api/v1/admin/diy/modeling/config" {
			handler = r
		}
	}
	customer, _ := common.GenAccessToken(secret, common.TokenInfo{UserId: 1, UserType: "user", Roles: []string{"customer"}}, 3600)
	shop, _ := common.GenAccessToken(secret, common.TokenInfo{UserId: 1, UserType: "admin", Roles: []string{"shop_admin"}}, 3600)
	wrongDomain, _ := common.GenAccessToken(secret, common.TokenInfo{UserId: 1, UserType: "user", Roles: []string{"platform_super"}}, 3600)
	service, _ := common.SignServiceToken(secret, "diy-service")
	for _, token := range []string{"", customer, shop, wrongDomain, service} {
		r := httptest.NewRequest("PUT", handler.Path, strings.NewReader(`{"apiKey":"must-never-save"}`))
		r.Header.Set("X-User-Roles", "platform_super")
		if token != "" {
			r.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		handler.Handler(w, r)
		var body struct {
			Code int `json:"code"`
		}
		if json.Unmarshal(w.Body.Bytes(), &body) != nil || body.Code == 0 {
			t.Fatal("unauthorized credential write", w.Body.String())
		}
	}
}

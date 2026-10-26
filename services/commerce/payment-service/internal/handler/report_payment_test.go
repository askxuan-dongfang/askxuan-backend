package handler

import (
	"encoding/json"
	"github.com/askxuan/common"
	"github.com/askxuan/payment-service/internal/svc"
	"github.com/zeromicro/go-zero/rest"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReportPointsEndpointRetiredWithoutDatabase(t *testing.T) {
	s := &svc.ServiceContext{}
	s.Config.Auth.AccessSecret = strings.Repeat("fixture", 8)
	server, err := rest.NewServer(rest.RestConf{})
	if err != nil {
		t.Fatal(err)
	}
	registerReportPayment(server, s)
	token, err := common.GenAccessToken(s.Config.Auth.AccessSecret, common.TokenInfo{UserId: 9501, UserType: "user", Roles: []string{"customer"}}, 3600)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/api/v1/payments/ai-report", strings.NewReader(`{"reportId":1,"expectedPoints":10}`))
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	server.Routes()[0].Handler(w, r)
	var response struct {
		Code int `json:"code"`
	}
	if json.Unmarshal(w.Body.Bytes(), &response) != nil || response.Code != 41001 {
		t.Fatal("legacy points purchase was not rejected", w.Body.String())
	}
}

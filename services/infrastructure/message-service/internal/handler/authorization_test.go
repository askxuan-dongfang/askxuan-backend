package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/askxuan/common"
	"github.com/askxuan/message-service/internal/config"
	"github.com/askxuan/message-service/internal/model"
	"github.com/askxuan/message-service/internal/svc"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/rest"
)

const messageTestSecret = "isolated-message-auth-test"

func messageTestServer(t *testing.T, ctx *svc.ServiceContext) *rest.Server {
	t.Helper()
	var c rest.RestConf
	if err := conf.FillDefault(&c); err != nil {
		t.Fatal(err)
	}
	c.Name = "message-authorization-test"
	c.Host = "127.0.0.1"
	server, err := rest.NewServer(c)
	if err != nil {
		t.Fatal(err)
	}
	ctx.Config = config.Config{AuthSecret: messageTestSecret}
	RegisterHandlers(server, ctx)
	return server // Never start a listener or construct real external dependencies.
}

func messageTestRequest(t *testing.T, method, path string, identity common.TokenInfo) *http.Request {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader("{}"))
	r.Header.Set("Content-Type", "application/json")
	if identity.UserId != 0 {
		token, err := common.GenAccessToken(messageTestSecret, identity, 60)
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("Authorization", "Bearer "+token)
	}
	// Neither a direct caller nor a proxy header can grant a different identity.
	r.Header.Set("X-User-Id", "999")
	r.Header.Set("X-User-Type", "admin")
	r.Header.Set("X-User-Roles", "platform_super,master")
	r.Header.Set("X-Master-Id", "999")
	return r
}

func TestRegisteredManagementRoutesRejectOtherIdentities(t *testing.T) {
	// Nil models ensure a failed guard cannot silently exercise a database or push operation.
	server := messageTestServer(t, &svc.ServiceContext{})
	routeCount := 0
	for _, route := range server.Routes() {
		if !strings.HasPrefix(route.Path, "/api/v1/admin/messages") && !strings.HasPrefix(route.Path, "/api/v1/admin/announcements") {
			continue
		}
		routeCount++
		isMaster := route.Path == "/api/v1/admin/messages/master" || strings.HasPrefix(route.Path, "/api/v1/admin/messages/master/")
		for _, identity := range []struct {
			name, kind, role string
			masterID         int64
		}{
			{"anonymous", "", "", 0}, {"customer", "user", "customer", 0},
			{"shop", "admin", "shop_admin", 0}, {"temple", "admin", "temple_admin", 0},
			{"human-service", "admin", "platform_service", 0}, {"internal-service", "service", "platform_service", 0},
			{"service-super", "service", "platform_super", 0}, {"customer-super", "user", "platform_super", 0},
			{"legacy-super", "", "platform_super", 0}, {"master-unbound", "admin", "master", 0},
			{"master", "admin", "master", 41}, {"platform-super", "admin", "platform_super", 0},
		} {
			if (isMaster && identity.name == "master") || (!isMaster && identity.name == "platform-super") {
				continue
			}
			t.Run(route.Method+route.Path+"/"+identity.name, func(t *testing.T) {
				id := int64(7)
				if identity.name == "anonymous" {
					id = 0
				}
				r := messageTestRequest(t, route.Method, route.Path, common.TokenInfo{UserId: id, UserType: identity.kind, Roles: []string{identity.role}, MasterID: identity.masterID})
				rr := httptest.NewRecorder()
				route.Handler(rr, r)
				var body common.Body
				if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
					t.Fatal(err)
				}
				want := common.ErrRoleForbidden.Code
				if identity.name == "anonymous" {
					want = common.ErrUnauthorized.Code
				}
				if body.Code != want {
					t.Fatalf("code=%d want=%d body=%s", body.Code, want, rr.Body.String())
				}
			})
		}
	}
	if routeCount != 10 {
		t.Fatalf("expected 10 management operations, checked %d", routeCount)
	}
}

type templateAuthFixture struct {
	model.TemplateModel
	calls int
}

func (m *templateAuthFixture) List(context.Context, string, int, int) ([]*model.MessageTemplate, int64, error) {
	m.calls++
	return []*model.MessageTemplate{}, 0, nil
}

type messageAuthFixture struct {
	model.MessageModel
	recipient string
	calls     int
	readID    int64
}

func (m *messageAuthFixture) List(_ context.Context, recipient string, _, _, _ int) ([]*model.Message, int64, error) {
	m.recipient, m.calls = recipient, m.calls+1
	return []*model.Message{}, 0, nil
}
func (m *messageAuthFixture) MarkReadByUser(_ context.Context, id int64, recipient string) error {
	m.readID, m.recipient, m.calls = id, recipient, m.calls+1
	return nil
}

func TestRegisteredManagementRoutesPreserveAuthorizedOperations(t *testing.T) {
	for _, tc := range []struct {
		name, method, path, role string
		masterID                 int64
	}{
		{"platform-templates", http.MethodGet, "/api/v1/admin/messages/templates", "platform_super", 0},
		{"master-inbox", http.MethodGet, "/api/v1/admin/messages/master", "master", 41},
		{"master-read", http.MethodPut, "/api/v1/admin/messages/master/123/read", "master", 41},
	} {
		t.Run(tc.name, func(t *testing.T) {
			templates, messages := &templateAuthFixture{}, &messageAuthFixture{}
			server := messageTestServer(t, &svc.ServiceContext{TemplateModel: templates, MessageModel: messages})
			r := messageTestRequest(t, tc.method, tc.path, common.TokenInfo{UserId: 7, UserType: "admin", Roles: []string{tc.role}, MasterID: tc.masterID})
			rr := httptest.NewRecorder()
			server.ServeHTTP(rr, r)
			var body common.Body
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil || body.Code != 0 {
				t.Fatalf("request failed: %v %s", err, rr.Body.String())
			}
			if tc.role == "platform_super" {
				if templates.calls != 1 || messages.calls != 0 {
					t.Fatal("wrong management operation invoked")
				}
			} else {
				if messages.calls != 1 || messages.recipient != "m_41" {
					t.Fatalf("wrong identity namespace: %+v", messages)
				}
				if tc.method == http.MethodPut && messages.readID != 123 {
					t.Fatalf("read message ID=%d", messages.readID)
				}
			}
		})
	}
}

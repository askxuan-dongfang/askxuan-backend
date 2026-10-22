package handler

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
)

func TestPayoutSandboxBodyRejectsAccountFieldsAndTrailingDocuments(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.CtxKeyUserType, "admin")
	ctx = context.WithValue(ctx, middleware.CtxKeyUserID, int64(9))
	ctx = context.WithValue(ctx, middleware.CtxKeyRoles, []string{"platform_super"})
	for _, body := range []string{`{"requestId":"00000000-0000-0000-0000-000000000001","amountCents":100,"bankCard":"forbidden"}`, `{"account":"forbidden"}`, `{"recipient":"forbidden"}`, `{"amountCents":1.5}`, `{} {}`, strings.Repeat(" ", 1100) + `{}`} {
		w := httptest.NewRecorder()
		payoutSandboxHandler(nil, "create")(w, httptest.NewRequest("POST", "/", strings.NewReader(body)).WithContext(ctx))
		var result struct {
			Code int `json:"code"`
		}
		if json.Unmarshal(w.Body.Bytes(), &result) != nil || result.Code != common.ErrParamInvalid.Code {
			t.Fatalf("unsafe body accepted: %s", w.Body.String())
		}
	}
}
func TestPayoutSandboxHandlerRejectsServiceIdentity(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.CtxKeyUserType, "service")
	ctx = context.WithValue(ctx, middleware.CtxKeyUserID, int64(9))
	ctx = context.WithValue(ctx, middleware.CtxKeyRoles, []string{"platform_super"})
	for _, action := range []string{"list", "detail", "create", "action"} {
		w := httptest.NewRecorder()
		payoutSandboxHandler(nil, action)(w, httptest.NewRequest("POST", "/", strings.NewReader(`{}`)).WithContext(ctx))
		var result struct {
			Code int `json:"code"`
		}
		if json.Unmarshal(w.Body.Bytes(), &result) != nil || result.Code != common.ErrForbidden.Code {
			t.Fatal("service identity entered human simulator")
		}
	}
}

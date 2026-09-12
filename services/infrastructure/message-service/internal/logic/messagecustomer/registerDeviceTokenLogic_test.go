package messagecustomer

import (
	"context"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/message-service/internal/model"
	"github.com/askxuan/message-service/internal/svc"
	"github.com/askxuan/message-service/internal/types"
	"strings"
	"testing"
)

type deviceCapture struct {
	model.DeviceTokenModel
	stored *model.DeviceToken
}

func (d *deviceCapture) Upsert(ctx context.Context, v *model.DeviceToken) (int64, error) {
	d.stored = v
	return 1, nil
}
func TestDeviceRegistrationUsesAuthenticatedIdentity(t *testing.T) {
	fake := &deviceCapture{}
	s := &svc.ServiceContext{DeviceTokenModel: fake}
	ctx := context.WithValue(context.Background(), middleware.CtxKeyUserID, int64(42))
	req := &types.DeviceTokenRegisterReq{UserId: "attacker-chosen-user", DeviceToken: strings.Repeat("ab", 32), BundleId: "com.dongfang.customer", Environment: "sandbox"}
	if _, err := NewRegisterDeviceTokenLogic(ctx, s).RegisterDeviceToken(req); err != nil {
		t.Fatal(err)
	}
	if fake.stored.UserId != "42" || fake.stored.ChatIdentity != "u_42" {
		t.Fatal("request body overrode authenticated device owner")
	}
	req.DeviceToken = "mock-token"
	if _, err := NewRegisterDeviceTokenLogic(ctx, s).RegisterDeviceToken(req); err == nil {
		t.Fatal("mock device token accepted")
	}
	req.DeviceToken = strings.Repeat("cd", 32)
	req.BundleId = "com.askxuan.master"
	if _, err := NewRegisterDeviceTokenLogic(ctx, s).RegisterDeviceToken(req); err == nil {
		t.Fatal("customer registered master app token")
	}
	ctx = context.WithValue(ctx, middleware.CtxKeyMasterID, int64(7))
	if _, err := NewRegisterDeviceTokenLogic(ctx, s).RegisterDeviceToken(req); err != nil {
		t.Fatal(err)
	}
	if fake.stored.ChatIdentity != "m_7" {
		t.Fatal("master push identity was incorrect")
	}
}

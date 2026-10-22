package logic

import (
	"context"
	"testing"

	"github.com/askxuan/common/middleware"
	"github.com/askxuan/finance-service/internal/types"
	"github.com/google/uuid"
)

func sandboxContext(kind string, roles ...string) context.Context {
	ctx := context.WithValue(context.Background(), middleware.CtxKeyUserType, kind)
	ctx = context.WithValue(ctx, middleware.CtxKeyUserID, int64(9))
	return context.WithValue(ctx, middleware.CtxKeyRoles, roles)
}
func TestPayoutSandboxRequiresHumanSuperBeforeAnyRepositoryAccess(t *testing.T) {
	for _, tc := range []struct{ kind, role string }{{"user", "customer"}, {"admin", "finance_reader"}, {"admin", "temple_admin"}, {"master", "master"}, {"service", "platform_super"}, {"user", "platform_super"}} {
		l := &PayoutSandboxLogic{ctx: sandboxContext(tc.kind, tc.role)}
		if _, err := l.List(&types.PayoutSandboxListReq{}); err == nil {
			t.Fatal("non-super list accepted")
		}
		if _, err := l.Detail("sandbox_" + uuid.NewString()); err == nil {
			t.Fatal("non-super detail accepted")
		}
		if _, err := l.Create(&types.PayoutSandboxCreateReq{}); err == nil {
			t.Fatal("non-super create accepted")
		}
		if _, err := l.Act("sandbox_"+uuid.NewString(), &types.PayoutSandboxActionReq{}); err == nil {
			t.Fatal("non-super mutation accepted")
		}
	}
}
func TestPayoutSandboxInputBoundsAndNoRealIDs(t *testing.T) {
	l := &PayoutSandboxLogic{ctx: sandboxContext("admin", "platform_super")}
	for _, amount := range []int64{0, -1, 1000001} {
		if _, err := l.Create(&types.PayoutSandboxCreateReq{RequestID: uuid.NewString(), AmountCents: amount}); err == nil {
			t.Fatal("amount outside simulator bounds accepted")
		}
	}
	for _, id := range []string{"WD20261022001", "123", "sandbox_bad", ""} {
		if _, err := l.Detail(id); err == nil {
			t.Fatal("non-sandbox ID accepted")
		}
	}
	for _, req := range []types.PayoutSandboxActionReq{{RequestID: uuid.NewString(), Version: 1, Action: "refund"}, {RequestID: uuid.NewString(), Version: 1, Action: "approve", Outcome: "success"}, {RequestID: uuid.NewString(), Version: 1, Action: "resolve", Outcome: "unknown"}, {RequestID: uuid.NewString(), Version: 0, Action: "submit", Outcome: "success"}} {
		if _, err := l.Act("sandbox_"+uuid.NewString(), &req); err == nil {
			t.Fatal("invalid sandbox command accepted")
		}
	}
}

package logic

import (
	"context"
	"github.com/askxuan/user-service/internal/svc"
	"github.com/askxuan/user-service/internal/types"
	"testing"
)

func TestLegacyRegistrationCannotCreateUnverifiedAccounts(t *testing.T) {
	// A nil model proves none of these old payloads reaches storage.
	l := NewRegisterLogic(context.Background(), &svc.ServiceContext{})
	for _, code := range []string{"", "1234", "999999"} {
		if result, e := l.Register(&types.RegisterReq{Mobile: "13900000028", Code: code}); e == nil || result != nil {
			t.Fatal("legacy registration allowed")
		}
	}
}

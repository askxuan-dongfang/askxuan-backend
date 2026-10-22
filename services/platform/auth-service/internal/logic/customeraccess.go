package logic

import (
	"context"
	"github.com/askxuan/auth-service/internal/svc"
	"github.com/askxuan/common"
)

// CurrentCustomerAccess reloads status and its durable revision on every human
// user request. It deliberately does not cache availability or infer an admin
// identity from the numeric ID. Reads failing are never permission to proceed.
func CurrentCustomerAccess(ctx context.Context, s *svc.ServiceContext, c *common.CustomClaims) (common.TokenInfo, error) {
	if c == nil || c.UserType != "user" || c.IsRefreshToken() || c.UserId <= 0 || c.ClientID != "customer" || len(c.Roles) != 1 || !c.HasRole("customer") {
		return common.TokenInfo{}, common.ErrTokenInvalid
	}
	return NewRefreshLogic(ctx, s).refreshIdentity(c)
}

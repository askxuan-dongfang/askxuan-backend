package handler

import (
	"net/http"
	"strconv"

	"github.com/askxuan/common"
	"github.com/askxuan/common/adminpolicy"
)

// The manual review interfaces have no internal-service caller. A signed human
// identity is required even on a direct connection; forwarded headers are ignored.
func humanAuditActor(r *http.Request, secret string, superOnly bool) (string, error) {
	claims, err := operationClaims(r, secret)
	if err != nil {
		return "", err
	}
	if claims.UserType != "admin" || claims.UserId <= 0 || claims.HasRole("platform_service") {
		return "", common.ErrForbidden
	}
	if claims.HasRole("platform_super") {
		return strconv.FormatInt(claims.UserId, 10), nil
	}
	if !superOnly && claims.ClientID == "platform-admin" && claims.AuthzScope == "platform" && adminpolicy.Allowed(r.Method, r.URL.Path, claims.Permissions) {
		return strconv.FormatInt(claims.UserId, 10), nil
	}
	return "", common.ErrForbidden
}

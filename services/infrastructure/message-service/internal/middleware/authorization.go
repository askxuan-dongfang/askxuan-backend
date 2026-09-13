package middleware

import (
	"net/http"

	"github.com/askxuan/common"
	commonauth "github.com/askxuan/common/middleware"
)

// AdminRoleFunc runs after JWT authentication and reads only verified claims.
// Internal notification consumers use MQ and do not require management HTTP access.
func AdminRoleFunc(role string) func(http.HandlerFunc) http.HandlerFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			userType, _ := r.Context().Value(commonauth.CtxKeyUserType).(string)
			if userType != "admin" || commonauth.UserIDFromCtx(r.Context()) <= 0 {
				common.JsonError(w, common.ErrRoleForbidden)
				return
			}
			if role == "master" && commonauth.MasterIDFromCtx(r.Context()) <= 0 {
				common.JsonError(w, common.ErrRoleForbidden)
				return
			}
			for _, actual := range commonauth.RolesFromCtx(r.Context()) {
				if actual == role {
					next(w, r)
					return
				}
			}
			common.JsonError(w, common.ErrRoleForbidden)
		}
	}
}

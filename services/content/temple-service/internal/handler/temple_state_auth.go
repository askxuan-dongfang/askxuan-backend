package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/askxuan/common"
	"github.com/askxuan/common/adminpolicy"
	"github.com/askxuan/temple-service/internal/svc"
)

type templeStateAccess struct {
	UserID      int64    `json:"userId"`
	Roles       []string `json:"roles"`
	ClientID    string   `json:"clientId"`
	AuthzScope  string   `json:"authzScope"`
	Permissions []string `json:"permissions"`
}

// Sensitive subject-state writes also defend direct service connections. The
// authority URL is fixed server configuration; no identity or target headers
// from the incoming request can choose an upstream or establish permission.
func authorizeTempleState(sc *svc.ServiceContext, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		access, err := templeStateAuthority(sc, r)
		if err != nil {
			common.JsonError(w, err)
			return
		}
		r.Header.Set("X-User-Id", strconv.FormatInt(access.UserID, 10))
		r.Header.Set("X-User-Roles", strings.Join(access.Roles, ","))
		r.Header.Set("X-User-Type", "admin")
		r.Header.Set("X-Client-Id", access.ClientID)
		for _, name := range []string{"X-Temple-Id", "X-Temple-Code", "X-Master-Id"} {
			r.Header.Del(name)
		}
		ctx := svc.WithTempleID(svc.WithUserID(r.Context(), access.UserID), 0)
		next(w, r.WithContext(ctx))
	}
}

func templeStateAuthority(sc *svc.ServiceContext, r *http.Request) (templeStateAccess, error) {
	var access templeStateAccess
	bearer := r.Header.Get("Authorization")
	if !strings.HasPrefix(bearer, "Bearer ") || strings.TrimSpace(strings.TrimPrefix(bearer, "Bearer ")) == "" {
		return access, common.ErrUnauthorized
	}
	if sc == nil {
		return access, common.ErrDependencyUnavailable
	}
	authority, err := url.Parse(sc.Config.AuthServiceURL)
	if err != nil || (authority.Scheme != "http" && authority.Scheme != "https") || authority.Host == "" || authority.User != nil || authority.RawQuery != "" || authority.Fragment != "" || (authority.Path != "" && authority.Path != "/") {
		return access, common.ErrDependencyUnavailable
	}
	authority.Path = "/api/v1/auth/access"
	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, authority.String(), nil)
	if err != nil {
		return access, common.ErrDependencyUnavailable
	}
	req.Header.Set("Authorization", bearer)
	client := &http.Client{Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		return access, common.ErrDependencyUnavailable
	}
	defer res.Body.Close()
	var envelope struct {
		Code *int              `json:"code"`
		Data templeStateAccess `json:"data"`
	}
	if res.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(res.Body, 1<<16)).Decode(&envelope) != nil || envelope.Code == nil {
		return access, common.ErrDependencyUnavailable
	}
	switch *envelope.Code {
	case 0:
	case common.ErrSessionRevoked.Code:
		return access, common.ErrSessionRevoked
	case common.ErrUnauthorized.Code, common.ErrTokenInvalid.Code, common.ErrTokenExpired.Code:
		return access, common.ErrTokenInvalid
	case common.ErrForbidden.Code, common.ErrRoleForbidden.Code, common.ErrUserDisabled.Code:
		return access, common.ErrRoleForbidden
	default:
		return access, common.ErrDependencyUnavailable
	}
	access = envelope.Data
	if access.UserID <= 0 || access.ClientID != "platform-admin" || access.AuthzScope != "platform" || len(access.Roles) != 1 || access.Roles[0] == "" {
		return templeStateAccess{}, common.ErrRoleForbidden
	}
	if access.Roles[0] == "platform_super" {
		return access, nil
	}
	if !adminpolicy.Builtin(access.Roles[0]) && adminpolicy.Allowed(r.Method, r.URL.Path, access.Permissions) {
		return access, nil
	}
	return templeStateAccess{}, common.ErrRoleForbidden
}

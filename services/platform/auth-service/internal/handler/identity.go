package handler

import (
	"encoding/json"
	"fmt"
	"github.com/askxuan/auth-service/internal/logic"
	"github.com/askxuan/auth-service/internal/svc"
	"github.com/askxuan/auth-service/internal/types"
	"github.com/askxuan/common"
	"github.com/askxuan/common/identity"
	"github.com/zeromicro/go-zero/rest"
	"net"
	"net/http"
	"os"
	"strings"
)

func identityError(w http.ResponseWriter, e error) {
	common.JsonError(w, &common.BizError{Code: 40010, Msg: e.Error()})
}

// Only explicitly trusted proxy hops may supply client addresses. Walk from
// the nearest hop so a public caller cannot forge a rate-limit identity.
func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	trusted := func(value string) bool {
		ip := net.ParseIP(strings.TrimSpace(value))
		if ip == nil {
			return false
		}
		for _, cidr := range strings.Split(os.Getenv("AUTH_TRUSTED_PROXY_CIDRS"), ",") {
			_, block, err := net.ParseCIDR(strings.TrimSpace(cidr))
			if err == nil && block.Contains(ip) {
				return true
			}
		}
		return false
	}
	if !trusted(host) {
		return host
	}
	chain := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(chain) - 1; i >= 0; i-- {
		candidate := strings.TrimSpace(chain[i])
		if net.ParseIP(candidate) == nil {
			return host
		}
		host = candidate
		if !trusted(host) {
			break
		}
	}
	return host
}
func checkHuman(w http.ResponseWriter, r *http.Request, sc *svc.ServiceContext, id, code string) bool {
	if sc.Accounts == nil {
		common.JsonError(w, common.ErrSystem)
		return false
	}
	if e := sc.Accounts.Challenges.Limit(r.Context(), "request-ip", remoteIP(r), 60, 600); e != nil {
		identityError(w, e)
		return false
	}
	if e := sc.Accounts.Challenges.Consume(r.Context(), "captcha", id, code, 1); e != nil {
		identityError(w, e)
		return false
	}
	return true
}

type identityRequest struct {
	Kind        string `json:"kind"`
	Domain      string `json:"domain"`
	Email       string `json:"email"`
	Purpose     string `json:"purpose"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	Code        string `json:"code"`
	Agreement   string `json:"agreementVersion"`
	CaptchaID   string `json:"captchaId"`
	CaptchaCode string `json:"captchaCode"`
}

func registerIdentityHandlers(server *rest.Server, sc *svc.ServiceContext) {
	server.AddRoutes([]rest.Route{
		{Method: "GET", Path: "/api/v1/auth/options", Handler: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			common.Ok(w, map[string]any{"passwordEnabled": true, "emailEnabled": sc.Accounts != nil && sc.Accounts.Mailer.Ready(), "smsEnabled": false, "agreementVersion": identity.AgreementVersion})
		}},
		{Method: "GET", Path: "/api/v1/auth/captcha", Handler: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			if sc.Accounts == nil {
				common.JsonError(w, common.ErrSystem)
				return
			}
			if e := sc.Accounts.Challenges.Limit(r.Context(), "captcha-ip", remoteIP(r), 60, 600); e != nil {
				identityError(w, e)
				return
			}
			c, e := sc.Accounts.Challenges.Captcha(r.Context())
			if e != nil {
				common.JsonError(w, common.ErrSystem)
				return
			}
			common.Ok(w, c)
		}},
	})
	for _, action := range []string{"email/code", "email/register", "password/reset", "work/register", "work/activate"} {
		action := action
		server.AddRoute(rest.Route{Method: "POST", Path: "/api/v1/auth/" + action, Handler: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			r.Body = http.MaxBytesReader(w, r.Body, 8192)
			var q identityRequest
			if json.NewDecoder(r.Body).Decode(&q) != nil {
				common.JsonError(w, common.ErrParam)
				return
			}
			if q.Domain == "" {
				q.Domain = "user"
			}
			if !checkHuman(w, r, sc, q.CaptchaID, q.CaptchaCode) {
				return
			}
			switch action {
			case "email/code":
				if e := sc.Accounts.SendCode(r.Context(), q.Domain, q.Email, q.Purpose); e != nil {
					identityError(w, e)
					return
				}
				common.Ok(w, map[string]any{"retryAfter": 60, "message": "若邮箱符合条件，验证码将发送到邮箱，请检查收件箱和垃圾邮件"})
			case "email/register":
				if q.Domain != "user" {
					identityError(w, fmt.Errorf("工作账户不开放注册"))
					return
				}
				id, e := sc.Accounts.Register(r.Context(), q.Email, q.Username, q.Password, q.Code, q.Agreement)
				if e != nil {
					identityError(w, e)
					return
				}
				resp, e := logic.NewLoginLogic(r.Context(), sc).IssueCustomer(id)
				if e != nil {
					common.JsonError(w, e)
					return
				}
				common.Ok(w, resp)
			case "work/register":
				_, e := sc.Accounts.RegisterWork(r.Context(), q.Kind, q.Email, q.Username, q.Password, q.Code, q.Agreement)
				if e != nil {
					identityError(w, e)
					return
				}
				resp, e := logic.NewAdminLoginLogic(r.Context(), sc).AdminLogin(&types.AdminLoginReq{Account: q.Email, Password: q.Password})
				if e != nil {
					common.JsonError(w, e)
					return
				}
				common.Ok(w, resp)
			case "work/activate":
				if e := sc.Accounts.ActivateManagedMaster(r.Context(), q.Email, q.Password, q.Code, q.Agreement); e != nil {
					identityError(w, e)
					return
				}
				common.Ok(w, map[string]bool{"success": true})
			case "password/reset":
				if !strings.Contains(q.Email, "@") {
					common.JsonError(w, common.ErrParam)
					return
				}
				if e := sc.Accounts.Reset(r.Context(), q.Domain, q.Email, q.Password, q.Code); e != nil {
					identityError(w, e)
					return
				}
				common.Ok(w, map[string]bool{"success": true})
			}
		}})
	}
}

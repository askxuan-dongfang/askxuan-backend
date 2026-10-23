package middleware

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/askxuan/common"
	"github.com/askxuan/common/adminpolicy"
)

// SecurityClient calls static internal upstreams directly, never the public gateway.
// It carries no user-supplied identity headers or body into access/audit requests.
type SecurityClient struct {
	AuthURL, AuditURL, Secret string
	Client                    *http.Client
}

func (s *SecurityClient) call(ctx context.Context, method, url, bearer string, payload any) error {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	req.Header.Set("Content-Type", "application/json")
	c := s.Client
	if c == nil {
		c = &http.Client{Timeout: 3 * time.Second}
	}
	res, err := c.Do(req)
	if err != nil {
		return errors.New("security upstream unavailable")
	}
	defer res.Body.Close()
	var out struct {
		Code *int `json:"code"`
	}
	if res.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&out) != nil || out.Code == nil || *out.Code != 0 {
		return errors.New("security upstream rejected request")
	}
	return nil
}
func (s *SecurityClient) ValidateAccess(ctx context.Context, bearer string, c *common.CustomClaims) error {
	if c == nil || (c.UserType != "admin" && c.UserType != "user") {
		return nil
	}
	if s.AuthURL == "" {
		return common.ErrDependencyUnavailable
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(s.AuthURL, "/")+"/api/v1/auth/access", nil)
	if err != nil {
		return common.ErrDependencyUnavailable
	}
	req.Header.Set("Authorization", "Bearer "+bearer)
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return common.ErrDependencyUnavailable
	}
	defer res.Body.Close()
	var out struct {
		Code *int `json:"code"`
	}
	if res.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&out) != nil || out.Code == nil {
		return common.ErrDependencyUnavailable
	}
	switch *out.Code {
	case 0:
		return nil
	case common.ErrSessionRevoked.Code:
		return common.ErrSessionRevoked
	case common.ErrTokenInvalid.Code, common.ErrUnauthorized.Code, common.ErrTokenExpired.Code, common.ErrUserDisabled.Code, common.ErrRoleForbidden.Code:
		return common.ErrTokenInvalid
	default:
		return common.ErrDependencyUnavailable
	}
}

type OperationStart struct {
	RequestID     string   `json:"requestId"`
	ActorID       int64    `json:"actorId"`
	ActorRoles    []string `json:"actorRoles"`
	Action        string   `json:"action"`
	Method        string   `json:"method"`
	RouteTemplate string   `json:"routeTemplate"`
	TargetType    string   `json:"targetType"`
	TargetID      string   `json:"targetId"`
}
type OperationFinish struct {
	RequestID    string `json:"requestId"`
	Result       string `json:"result"`
	HTTPStatus   int    `json:"httpStatus"`
	BusinessCode int    `json:"businessCode"`
	DurationMs   int64  `json:"durationMs"`
}
type OperationRecorder interface {
	Start(context.Context, OperationStart) error
	Finish(context.Context, OperationFinish) error
}

func (s *SecurityClient) record(ctx context.Context, stage string, payload any) error {
	if s.AuditURL == "" {
		return errors.New("audit target required")
	}
	token, err := common.SignServiceToken(s.Secret, "gateway-service")
	if err != nil {
		return err
	}
	return s.call(ctx, http.MethodPost, strings.TrimRight(s.AuditURL, "/")+"/api/v1/internal/admin-operations/"+stage, token, payload)
}
func (s *SecurityClient) Start(ctx context.Context, event OperationStart) error {
	return s.record(ctx, "start", event)
}
func (s *SecurityClient) Finish(ctx context.Context, event OperationFinish) error {
	return s.record(ctx, "finish", event)
}

func adminWrite(r *http.Request, c *common.CustomClaims) bool {
	if c.UserType != "admin" || c.UserId <= 0 {
		return false
	}
	switch r.Method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
	default:
		return false
	}
	p := r.URL.Path
	return strings.HasPrefix(p, "/api/v1/admin/") || p == "/api/v1/ai/admin" || strings.HasPrefix(p, "/api/v1/ai/admin/") || strings.HasPrefix(p, "/api/v1/payments/growth/admin/") || strings.HasPrefix(p, "/api/v1/auth/onboarding/admin/")
}

var safeBusinessID = regexp.MustCompile(`^(?:[1-9][0-9]{0,18}|[A-Z][A-Z0-9_-]{0,63}|sandbox_[a-f0-9-]{36}|[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12})$`)
var safeRoleCode = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

func operationFor(r *http.Request, c *common.CustomClaims) (OperationStart, error) {
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return OperationStart{}, err
	}
	e := OperationStart{RequestID: hex.EncodeToString(id), ActorID: c.UserId, ActorRoles: []string{}, Method: r.Method, Action: "admin.write", RouteTemplate: "/api/v1/admin/*", TargetType: "admin"}
	for _, role := range c.Roles {
		if safeRoleCode.MatchString(role) {
			e.ActorRoles = append(e.ActorRoles, role)
		}
	}
	if strings.HasPrefix(r.URL.Path, "/api/v1/ai/admin") {
		e.Action = "ai.admin.write"
		e.RouteTemplate = "/api/v1/ai/admin/*"
	}
	if strings.HasPrefix(r.URL.Path, "/api/v1/payments/growth/admin/") {
		e.Action = "growth.admin.write"
		e.RouteTemplate = "/api/v1/payments/growth/admin/*"
	}
	if strings.HasPrefix(r.URL.Path, "/api/v1/auth/onboarding/") {
		e.Action = "onboarding.admin.write"
		e.RouteTemplate = "/api/v1/auth/onboarding/*"
	}
	if route, ok := adminpolicy.Match(r.Method, r.URL.Path); ok {
		e.RouteTemplate = route.Template
		if route.Permission != "" {
			e.Action = route.Permission
		}
		a, b := strings.Split(route.Template, "/"), strings.Split(r.URL.Path, "/")
		for i, segment := range a {
			if segment == ":id" && safeBusinessID.MatchString(b[i]) {
				e.TargetID = b[i]
			}
		}
		// Resource comes from catalog metadata, never a user-controlled URL segment.
		for _, cap := range adminpolicy.Catalog {
			if cap.Code == route.Permission {
				e.TargetType = cap.Resource
				break
			}
		}
		if strings.HasPrefix(route.Template, "/api/v1/admin/auth/accounts") {
			e.TargetType = "admin_account"
		}
		if strings.HasPrefix(route.Template, "/api/v1/admin/auth/roles") {
			e.TargetType = "admin_role"
		}
	}
	return e, nil
}

// Buffer a normal management response until audit finalize so a successful write
// can carry a pending marker without changing its business result. The bounded
// overflow/Flush path streams with the pending marker set in advance.
type operationResponse struct {
	dest      http.ResponseWriter
	header    http.Header
	status    int
	body      bytes.Buffer
	prefix    bytes.Buffer
	streaming bool
}

func newOperationResponse(w http.ResponseWriter) *operationResponse {
	return &operationResponse{dest: w, header: make(http.Header)}
}
func (w *operationResponse) Header() http.Header { return w.header }
func (w *operationResponse) WriteHeader(status int) {
	if w.status == 0 {
		w.status = status
	}
}
func (w *operationResponse) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if w.prefix.Len() < 65536 {
		n := 65536 - w.prefix.Len()
		if n > len(b) {
			n = len(b)
		}
		_, _ = w.prefix.Write(b[:n])
	}
	if !w.streaming && w.body.Len()+len(b) <= 8<<20 {
		return w.body.Write(b)
	}
	if !w.streaming {
		w.flushPending()
	}
	return w.dest.Write(b)
}
func (w *operationResponse) flushPending() {
	if w.streaming {
		return
	}
	w.streaming = true
	w.header.Set("X-Admin-Audit-Pending", "true")
	w.copyHeaders()
	w.dest.WriteHeader(w.statusCode())
	_, _ = w.dest.Write(w.body.Bytes())
	w.body.Reset()
}
func (w *operationResponse) Flush() {
	w.flushPending()
	if f, ok := w.dest.(http.Flusher); ok {
		f.Flush()
	}
}
func (w *operationResponse) statusCode() int {
	if w.status == 0 {
		return http.StatusOK
	}
	return w.status
}
func (w *operationResponse) copyHeaders() {
	for k, v := range w.header {
		w.dest.Header()[k] = append([]string(nil), v...)
	}
}
func (w *operationResponse) send(pending bool) {
	if w.streaming {
		return
	}
	if pending {
		w.header.Set("X-Admin-Audit-Pending", "true")
	}
	w.copyHeaders()
	w.dest.WriteHeader(w.statusCode())
	_, _ = w.dest.Write(w.body.Bytes())
}
func (w *operationResponse) outcome() (string, int) {
	var envelope struct {
		Code *int `json:"code"`
	}
	// Response bodies are transient bytes only; no body is written into audit or logs.
	decoder := json.NewDecoder(bytes.NewReader(w.prefix.Bytes()))
	if token, err := decoder.Token(); err == nil && token == json.Delim('{') {
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				break
			}
			if key == "code" {
				var code int
				if decoder.Decode(&code) == nil {
					envelope.Code = &code
				}
				break
			}
			var skip json.RawMessage
			if decoder.Decode(&skip) != nil {
				break
			}
		}
	}
	if envelope.Code == nil {
		if w.statusCode() >= 400 {
			return "failed", -1
		}
		return "unknown", -1
	}
	code := *envelope.Code
	if (code >= 40101 && code <= 40199) || (code >= 40301 && code <= 40399) || w.statusCode() == http.StatusUnauthorized || w.statusCode() == http.StatusForbidden {
		return "denied", code
	}
	if w.statusCode() >= 400 || code != 0 {
		return "failed", code
	}
	return "success", code
}
func auditBegin(ctx context.Context, r *http.Request, c *common.CustomClaims, recorder OperationRecorder, w http.ResponseWriter) (http.ResponseWriter, func(), error) {
	if recorder == nil || !adminWrite(r, c) {
		return w, func() {}, nil
	}
	e, err := operationFor(r, c)
	if err != nil {
		return w, nil, err
	}
	if err = recorder.Start(ctx, e); err != nil {
		return w, nil, fmt.Errorf("audit start unavailable")
	}
	out := newOperationResponse(w)
	started := time.Now()
	return out, func() {
		result, code := out.outcome()
		// Client disconnect must not prevent finalization after an executed mutation.
		finalCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 3*time.Second)
		defer cancel()
		err := recorder.Finish(finalCtx, OperationFinish{RequestID: e.RequestID, Result: result, HTTPStatus: out.statusCode(), BusinessCode: code, DurationMs: time.Since(started).Milliseconds()})
		out.send(err != nil)
	}, nil
}

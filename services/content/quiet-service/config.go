package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func authEndpoint(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil || (strings.ToLower(u.Scheme) != "http" && strings.ToLower(u.Scheme) != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return "", errors.New("静心账号鉴权地址配置无效")
	}
	u.Scheme, u.Host = strings.ToLower(u.Scheme), strings.ToLower(u.Host)
	if u.Scheme == "http" && u.Port() == "80" || u.Scheme == "https" && u.Port() == "443" {
		u.Host = strings.TrimSuffix(u.Host, ":"+u.Port())
	}
	u.Path, u.RawPath = "", ""
	return u.String(), nil
}

type accountError struct {
	status  int
	code    int
	message string
}

func (e *accountError) Error() string { return e.message }
func (e *accountError) write(w http.ResponseWriter) {
	writeJSON(w, e.status, map[string]any{"code": e.code, "message": e.message})
}

func accountUnavailable() *accountError {
	return &accountError{503, 50205, "账号历史鉴权暂不可用，未改存为访客记录。"}
}

// Only the auth service's live customer identity can select an account owner.
func (a *app) accountOwner(req *http.Request) (string, *accountError) {
	raw := req.Header.Get("Authorization")
	if a.authURL == "" {
		return "", &accountError{503, 50205, "当前未配置账号历史，请使用明确标注的访客模式。"}
	}
	if !strings.HasPrefix(raw, "Bearer ") || len(raw) <= 7 || len(raw) > 8192 {
		return "", &accountError{401, 40102, "登录凭证无效，请重新登录。"}
	}
	request, err := http.NewRequestWithContext(req.Context(), http.MethodGet, a.authURL+"/api/v1/auth/access", nil)
	if err != nil {
		return "", accountUnavailable()
	}
	request.Header.Set("Authorization", raw)
	client := a.authClient
	if client == nil {
		client = &http.Client{Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	response, err := client.Do(request)
	if err != nil {
		return "", accountUnavailable()
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 16385))
	var result struct {
		Code *int `json:"code"`
		Data struct {
			UserID   int64    `json:"userId"`
			Roles    []string `json:"roles"`
			ClientID string   `json:"clientId"`
		} `json:"data"`
	}
	if err != nil || len(body) > 16384 || response.StatusCode != http.StatusOK || json.Unmarshal(body, &result) != nil || result.Code == nil {
		return "", accountUnavailable()
	}
	switch *result.Code {
	case 0:
		if result.Data.UserID <= 0 || result.Data.ClientID != "customer" || len(result.Data.Roles) != 1 || result.Data.Roles[0] != "customer" {
			return "", &accountError{403, 40301, "仅信众账号可使用账号练习历史。"}
		}
		return tokenHash("quiet:customer:" + a.authURL + ":" + strconv.FormatInt(result.Data.UserID, 10)), nil
	case 40101, 40102, 40103, 40105:
		return "", &accountError{401, *result.Code, "登录已失效，请重新登录。"}
	case 40301, 40302, 40303, 40304:
		return "", &accountError{403, *result.Code, "当前账号无权使用账号练习历史。"}
	default:
		return "", accountUnavailable()
	}
}

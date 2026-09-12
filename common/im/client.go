package im

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Client OpenIM REST API 客户端（server-to-server）
// 所有方法失败时返回 error 但不 panic；调用方应 best-effort 处理。
type Client struct {
	apiURL       string
	adminUserID  string
	secret       string
	adminToken   string
	tokenMu      sync.Mutex
	tokenExpires time.Time
	httpClient   *http.Client
}

// NewClient 创建 OpenIM 客户端
func NewClient(apiURL, adminUserID, secret string) *Client {
	return &Client{
		apiURL:      apiURL,
		adminUserID: adminUserID,
		secret:      secret,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
	}
}

// GetAdminToken 获取管理员 token（POST /auth/get_admin_token）
func (c *Client) GetAdminToken(ctx context.Context) (string, error) {
	c.tokenMu.Lock()
	defer c.tokenMu.Unlock()
	if c.adminToken != "" && time.Now().Before(c.tokenExpires) {
		return c.adminToken, nil
	}
	req := GetAdminTokenReq{Secret: c.secret, UserID: c.adminUserID}
	var resp struct {
		ErrCode int    `json:"errCode"`
		ErrMsg  string `json:"errMsg"`
		Token   string `json:"token"`
		Data    struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	if err := c.post(ctx, "/auth/get_admin_token", req, &resp); err != nil {
		return "", err
	}
	if resp.ErrCode != 0 {
		return "", fmt.Errorf("openIM err: %s", resp.ErrMsg)
	}
	token := resp.Token
	if token == "" {
		token = resp.Data.Token
	}
	if token == "" {
		return "", fmt.Errorf("OpenIM returned an empty admin token")
	}
	c.adminToken = token
	c.tokenExpires = time.Now().Add(5 * time.Minute)
	return token, nil
}

// RegisterUser 注册用户到 OpenIM（POST /user/user_register，幂等）
func (c *Client) RegisterUser(ctx context.Context, userID, nickname, faceURL string) error {
	req := UserRegisterReq{Users: []OpenIMUser{{UserID: userID, Nickname: nickname, FaceURL: faceURL}}}
	var resp struct {
		ErrCode int    `json:"errCode"`
		ErrMsg  string `json:"errMsg"`
	}
	if err := c.postWithToken(ctx, "/user/user_register", req, &resp); err != nil {
		return err
	}
	if resp.ErrCode != 0 {
		return fmt.Errorf("openIM err: %s", resp.ErrMsg)
	}
	return nil
}

// GetUserToken 获取用户 IM token（POST /auth/get_user_token）
func (c *Client) GetUserToken(ctx context.Context, userID string) (string, error) {
	return c.GetUserTokenForPlatform(ctx, userID, 1)
}

func (c *Client) GetUserTokenForPlatform(ctx context.Context, userID string, platform int) (string, error) {
	req := UserTokenReq{UserID: userID, PlatformID: platform}
	var resp struct {
		ErrCode int           `json:"errCode"`
		ErrMsg  string        `json:"errMsg"`
		Token   string        `json:"token"`
		Data    UserTokenResp `json:"data"`
	}
	if err := c.postWithToken(ctx, "/auth/get_user_token", req, &resp); err != nil {
		return "", err
	}
	if resp.ErrCode != 0 {
		return "", fmt.Errorf("openIM err: %s", resp.ErrMsg)
	}
	if resp.Token != "" {
		return resp.Token, nil
	}
	return resp.Data.Token, nil
}

// SendMessage 服务端主动发消息（POST /msg/send_msg）
func (c *Client) SendMessage(ctx context.Context, req *SendMsgReq) error {
	var resp struct {
		ErrCode int    `json:"errCode"`
		ErrMsg  string `json:"errMsg"`
	}
	if err := c.postWithToken(ctx, "/msg/send_msg", req, &resp); err != nil {
		return err
	}
	if resp.ErrCode != 0 {
		return fmt.Errorf("openIM err: %s", resp.ErrMsg)
	}
	return nil
}

// post 发送 POST 请求（无需 admin token）
func (c *Client) post(ctx context.Context, path string, body interface{}, resp interface{}) error {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}
	url := c.apiURL + path
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("operationID", fmt.Sprintf("askxuan-%d", time.Now().UnixNano()))
	return c.do(req, resp)
}

// postWithToken 发送 POST 请求（带 admin token）
func (c *Client) postWithToken(ctx context.Context, path string, body interface{}, resp interface{}) error {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return err
	}
	url := c.apiURL + path
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("operationID", fmt.Sprintf("askxuan-%d", time.Now().UnixNano()))
	token, err := c.GetAdminToken(ctx)
	if err != nil {
		return err
	}
	req.Header.Set("token", token)
	return c.do(req, resp)
}

func (c *Client) do(req *http.Request, resp interface{}) error {
	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(httpResp.Body, 1<<20))
	if err != nil {
		return err
	}
	var envelope struct {
		ErrCode int `json:"errCode"`
	}
	_ = json.Unmarshal(data, &envelope)
	if httpResp.StatusCode == 401 || httpResp.StatusCode == 403 || envelope.ErrCode != 0 {
		// A failed authorized request may indicate an expired/revoked token. The
		// durable caller retries with a fresh credential; never replay sends here.
		if req.Header.Get("token") != "" {
			c.tokenMu.Lock()
			if c.adminToken == req.Header.Get("token") {
				c.adminToken = ""
				c.tokenExpires = time.Time{}
			}
			c.tokenMu.Unlock()
		}
	}
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return fmt.Errorf("OpenIM HTTP status %d", httpResp.StatusCode)
	}
	return json.Unmarshal(data, resp)
}

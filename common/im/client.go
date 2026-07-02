package im

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client OpenIM REST API 客户端（server-to-server）
// 所有方法失败时返回 error 但不 panic；调用方应 best-effort 处理。
type Client struct {
	apiURL      string
	adminUserID string
	secret      string
	adminToken  string
	httpClient  *http.Client
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
	req := GetAdminTokenReq{Secret: c.secret, UserID: c.adminUserID}
	var resp struct {
		ErrCode int               `json:"errCode"`
		ErrMsg  string            `json:"errMsg"`
		Data    GetAdminTokenResp `json:"data"`
	}
	if err := c.post(ctx, "/auth/get_admin_token", req, &resp); err != nil {
		return "", err
	}
	if resp.ErrCode != 0 {
		return "", fmt.Errorf("openIM err: %s", resp.ErrMsg)
	}
	c.adminToken = resp.Data.Token
	return resp.Data.Token, nil
}

// RegisterUser 注册用户到 OpenIM（POST /user/user_register，幂等）
func (c *Client) RegisterUser(ctx context.Context, userID, nickname, faceURL string) error {
	if c.adminToken == "" {
		if _, err := c.GetAdminToken(ctx); err != nil {
			return err
		}
	}
	req := UserRegisterReq{UserID: userID, Nickname: nickname, FaceURL: faceURL}
	var resp struct {
		ErrCode int    `json:"errCode"`
		ErrMsg  string `json:"errMsg"`
	}
	return c.postWithToken(ctx, "/user/user_register", req, &resp)
}

// GetUserToken 获取用户 IM token（POST /auth/user_token）
func (c *Client) GetUserToken(ctx context.Context, userID string) (string, error) {
	req := UserTokenReq{UserID: userID, Secret: c.secret}
	var resp struct {
		ErrCode int           `json:"errCode"`
		ErrMsg  string        `json:"errMsg"`
		Data    UserTokenResp `json:"data"`
	}
	if err := c.post(ctx, "/auth/user_token", req, &resp); err != nil {
		return "", err
	}
	if resp.ErrCode != 0 {
		return "", fmt.Errorf("openIM err: %s", resp.ErrMsg)
	}
	return resp.Data.Token, nil
}

// post 发送 POST 请求（无需 admin token）
func (c *Client) post(ctx context.Context, path string, body interface{}, resp interface{}) error {
	jsonBody, _ := json.Marshal(body)
	url := c.apiURL + path
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, resp)
}

// postWithToken 发送 POST 请求（带 admin token）
func (c *Client) postWithToken(ctx context.Context, path string, body interface{}, resp interface{}) error {
	jsonBody, _ := json.Marshal(body)
	url := c.apiURL + path
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("token", c.adminToken)
	return c.do(req, resp)
}

func (c *Client) do(req *http.Request, resp interface{}) error {
	httpResp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer httpResp.Body.Close()
	return json.NewDecoder(httpResp.Body).Decode(resp)
}

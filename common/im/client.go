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
	c.adminToken = token
	return token, nil
}

// RegisterUser 注册用户到 OpenIM（POST /user/user_register，幂等）
func (c *Client) RegisterUser(ctx context.Context, userID, nickname, faceURL string) error {
	if c.adminToken == "" {
		if _, err := c.GetAdminToken(ctx); err != nil {
			return err
		}
	}
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
	if c.adminToken == "" {
		if _, err := c.GetAdminToken(ctx); err != nil {
			return "", err
		}
	}
	req := UserTokenReq{UserID: userID, PlatformID: 1}
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
	if c.adminToken == "" {
		if _, err := c.GetAdminToken(ctx); err != nil {
			return err
		}
	}
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
	jsonBody, _ := json.Marshal(body)
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
	jsonBody, _ := json.Marshal(body)
	url := c.apiURL + path
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("operationID", fmt.Sprintf("askxuan-%d", time.Now().UnixNano()))
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

// Package weknora adapts the pinned v0.8.2 engine without exposing its credentials or API surface.
package weknora

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var ErrUnavailable = errors.New("WeKnora 暂不可用，请检查引擎与索引状态")
var ErrInput = errors.New("知识库参数无效或资料不属于此知识库")
var ErrConflict = errors.New("资料已变更，请刷新后重试")

type Client struct {
	base, key, embedding string
	http                 *http.Client
}
type Envelope struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data"`
	Total   int             `json:"total"`
}

func New(base, key, embedding string) (*Client, error) {
	u, e := url.Parse(base)
	if e != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || key == "" || embedding == "" {
		return nil, ErrInput
	}
	return &Client{strings.TrimRight(base, "/"), key, embedding, &http.Client{Timeout: 45 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func FromEnv() *Client {
	base := os.Getenv("AI_WEKNORA_URL")
	if base == "" {
		return nil
	}
	c, e := New(base, os.Getenv("AI_WEKNORA_KEY"), os.Getenv("AI_WEKNORA_EMBEDDING_ID"))
	if e != nil {
		panic("invalid WeKnora configuration")
	}
	return c
}
func (c *Client) call(ctx context.Context, method, path string, body io.Reader, contentType string) (Envelope, error) {
	var out Envelope
	b, e := c.raw(ctx, method, path, body, contentType)
	if e != nil {
		return out, e
	}
	if json.Unmarshal(b, &out) != nil || !out.Success {
		return out, ErrUnavailable
	}
	return out, nil
}

// raw also supports Wiki endpoints, whose responses are not envelopes.
func (c *Client) raw(ctx context.Context, method, path string, body io.Reader, contentType string) (json.RawMessage, error) {
	var out json.RawMessage
	req, e := http.NewRequestWithContext(ctx, method, c.base+"/api/v1"+path, body)
	if e != nil {
		return out, ErrInput
	}
	req.Header.Set("X-API-Key", c.key)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	r, e := c.http.Do(req)
	if e != nil {
		return out, ErrUnavailable
	}
	defer r.Body.Close()
	if r.StatusCode == 409 {
		return out, ErrConflict
	}
	if r.StatusCode < 200 || r.StatusCode >= 300 {
		return out, fmt.Errorf("%w (HTTP %d)", ErrUnavailable, r.StatusCode)
	}
	if r.StatusCode == http.StatusNoContent {
		return json.RawMessage(`{}`), nil
	}
	b, e := io.ReadAll(io.LimitReader(r.Body, 4*1024*1024+1))
	if e != nil || len(b) > 4*1024*1024 || !json.Valid(b) {
		return out, ErrUnavailable
	}
	return b, nil
}
func (c *Client) json(ctx context.Context, method, path string, payload any) (Envelope, error) {
	var body io.Reader
	if payload != nil {
		b, e := json.Marshal(payload)
		if e != nil {
			return Envelope{}, ErrInput
		}
		body = bytes.NewReader(b)
	}
	return c.call(ctx, method, path, body, "application/json")
}
func decode[T any](e Envelope, err error) (T, error) {
	var v T
	if err != nil {
		return v, err
	}
	if json.Unmarshal(e.Data, &v) != nil {
		return v, ErrUnavailable
	}
	return v, nil
}

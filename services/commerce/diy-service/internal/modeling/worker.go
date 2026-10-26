package modeling

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/askxuan/common"
)

func (m *Manager) Start(ctx context.Context) {
	go func() {
		timer := time.NewTicker(10 * time.Second)
		defer timer.Stop()
		for {
			m.Poll(ctx)
			select {
			case <-ctx.Done():
				return
			case <-timer.C:
			}
		}
	}()
}
func (m *Manager) Poll(ctx context.Context) {
	// A lost create response must never incur another billed generation automatically.
	_, _ = m.DB.ExecCtx(ctx, `UPDATE diy_modeling_task SET status='failed',error_message='生成提交结果未知，请到供应商核对后再重试' WHERE status='submitting' AND updated_at<DATE_SUB(NOW(),INTERVAL 5 MINUTE)`)
	_, _ = m.DB.ExecCtx(ctx, `UPDATE diy_modeling_task SET status='failed',error_message='生成任务超时，请核对供应商任务后重新提交' WHERE status IN ('queued','processing') AND created_at<DATE_SUB(NOW(),INTERVAL 2 HOUR)`)
	var rows []*Task
	if e := m.DB.QueryRowsPartialCtx(ctx, &rows, "SELECT "+taskFields+" FROM diy_modeling_task WHERE status IN ('queued','processing') AND next_poll_at<=NOW() ORDER BY created_at LIMIT 8"); e != nil {
		return
	}
	for _, t := range rows {
		if ctx.Err() != nil {
			return
		}
		m.process(ctx, t)
	}
}
func (m *Manager) fail(ctx context.Context, id, message string) {
	_, _ = m.DB.ExecCtx(ctx, "UPDATE diy_modeling_task SET status='failed',error_message=? WHERE id=? AND status<>'succeeded'", message, id)
}
func (m *Manager) retry(ctx context.Context, t *Task, message string) {
	if t.Attempts >= 10 {
		m.fail(ctx, t.ID, message)
		return
	}
	_, _ = m.DB.ExecCtx(ctx, "UPDATE diy_modeling_task SET attempts=attempts+1,next_poll_at=DATE_ADD(NOW(),INTERVAL 30 SECOND) WHERE id=? AND status='processing'", t.ID)
}
func (m *Manager) process(ctx context.Context, t *Task) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	cfg, e := m.readConfig(ctx)
	if e != nil {
		return
	}
	key, e := m.open(cfg.Cipher)
	if e != nil || key == "" {
		if t.Status == "queued" {
			m.fail(ctx, t.ID, "图生 3D 服务尚未配置")
		}
		return
	}
	if t.Status == "queued" {
		result, e := m.DB.ExecCtx(ctx, "UPDATE diy_modeling_task SET status='submitting' WHERE id=? AND status='queued'", t.ID)
		if e != nil {
			return
		}
		n, _ := result.RowsAffected()
		if n != 1 {
			return
		}
		var photos []string
		if json.Unmarshal([]byte(t.Photos), &photos) != nil {
			m.fail(ctx, t.ID, "素材记录无效")
			return
		}
		images := make([]string, 0, len(photos))
		for _, p := range photos {
			data, mime, e := m.photo(ctx, p)
			if e != nil {
				m.fail(ctx, t.ID, "实拍读取失败，仅支持本平台上传的 JPG、PNG、WebP 图片，单张最大 10MB")
				return
			}
			images = append(images, "data:"+mime+";base64,"+base64.StdEncoding.EncodeToString(data))
		}
		body := map[string]any{"image_urls": images, "ai_model": t.ModelName, "should_texture": true, "enable_pbr": true, "target_formats": []string{"glb", "usdz"}, "should_remesh": true, "target_polycount": 10000, "topology": "triangle", "image_enhancement": false}
		var created struct {
			Result string `json:"result"`
		}
		if m.provider(ctx, http.MethodPost, "/openapi/v1/multi-image-to-3d", key, body, &created) != nil || created.Result == "" || len(created.Result) > 128 {
			m.fail(ctx, t.ID, "生成提交未确认，请到供应商核对后再重试，系统不会自动重复扣费")
			return
		}
		_, _ = m.DB.ExecCtx(ctx, "UPDATE diy_modeling_task SET status='processing',provider_id=?,next_poll_at=DATE_ADD(NOW(),INTERVAL 10 SECOND) WHERE id=? AND status='submitting'", created.Result, t.ID)
		return
	}
	lease, e := m.DB.ExecCtx(ctx, "UPDATE diy_modeling_task SET next_poll_at=DATE_ADD(NOW(),INTERVAL 300 SECOND) WHERE id=? AND status='processing' AND next_poll_at<=NOW()", t.ID)
	if e != nil {
		return
	}
	n, _ := lease.RowsAffected()
	if n != 1 {
		return
	}
	var out struct {
		Status    string            `json:"status"`
		Progress  int               `json:"progress"`
		ModelURLs map[string]string `json:"model_urls"`
	}
	if m.provider(ctx, http.MethodGet, "/openapi/v1/multi-image-to-3d/"+url.PathEscape(t.ProviderID), key, nil, &out) != nil {
		m.retry(ctx, t, "供应商状态读取失败，请核对服务配置")
		return
	}
	switch out.Status {
	case "PENDING", "IN_PROGRESS":
		progress := max(0, min(99, out.Progress))
		_, _ = m.DB.ExecCtx(ctx, "UPDATE diy_modeling_task SET progress=?,attempts=0,next_poll_at=DATE_ADD(NOW(),INTERVAL 10 SECOND) WHERE id=? AND status='processing'", progress, t.ID)
	case "FAILED", "CANCELED":
		m.fail(ctx, t.ID, "供应商未能生成模型，请检查素材后重新提交")
	case "SUCCEEDED":
		stored := map[string]string{}
		for _, format := range []string{"glb", "usdz"} {
			data, e := m.DownloadModel(ctx, out.ModelURLs[format])
			if e != nil {
				m.retry(ctx, t, "模型下载失败或超过 32MB，请降低复杂度后重试")
				return
			}
			link, e := m.store(ctx, t.ID, format, data)
			if e != nil {
				m.retry(ctx, t, "模型入库失败或文件不符合安全要求")
				return
			}
			stored[format] = link
		}
		_, _ = m.DB.ExecCtx(ctx, "UPDATE diy_modeling_task SET status='succeeded',progress=100,glb_url=?,usdz_url=?,error_message='' WHERE id=? AND status='processing'", stored["glb"], stored["usdz"], t.ID)
	default:
		m.retry(ctx, t, "供应商返回未知任务状态")
	}
}
func (m *Manager) provider(ctx context.Context, method, path, key string, input, output any) error {
	var reader io.Reader
	if input != nil {
		raw, e := json.Marshal(input)
		if e != nil {
			return e
		}
		reader = bytes.NewReader(raw)
	}
	req, e := http.NewRequestWithContext(ctx, method, m.ProviderURL+path, reader)
	if e != nil {
		return e
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	res, e := m.Client.Do(req)
	if e != nil {
		return errors.New("model provider unavailable")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return errors.New("model provider rejected request")
	}
	return json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(output)
}
func (m *Manager) internalRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	token, e := common.SignServiceToken(m.Secret, "diy-service")
	if e != nil {
		return nil, e
	}
	req, e := http.NewRequestWithContext(ctx, method, m.FileURL+path, body)
	if e != nil {
		return nil, e
	}
	req.Header.Set("Authorization", "Bearer "+token)
	return m.Client.Do(req)
}
func (m *Manager) photo(ctx context.Context, source string) ([]byte, string, error) {
	res, e := m.internalRequest(ctx, http.MethodGet, "/internal/diy-model-photos?url="+url.QueryEscape(source), nil)
	if e != nil {
		return nil, "", e
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, "", errors.New("photo unavailable")
	}
	data, e := io.ReadAll(io.LimitReader(res.Body, (10<<20)+1))
	mime := http.DetectContentType(data)
	if e != nil || len(data) > 10<<20 || (mime != "image/jpeg" && mime != "image/png" && mime != "image/webp") {
		return nil, "", errors.New("photo invalid")
	}
	return data, mime, nil
}
func (m *Manager) store(ctx context.Context, id, format string, data []byte) (string, error) {
	res, e := m.internalRequest(ctx, http.MethodPut, "/internal/diy-models/"+id+"/"+format, bytes.NewReader(data))
	if e != nil {
		return "", e
	}
	defer res.Body.Close()
	var out struct {
		Code *int `json:"code"`
		Data struct {
			URL string `json:"url"`
		} `json:"data"`
	}
	if res.StatusCode != 200 || json.NewDecoder(io.LimitReader(res.Body, 4096)).Decode(&out) != nil || out.Code == nil || *out.Code != 0 || out.Data.URL == "" {
		return "", errors.New("model persistence failed")
	}
	return out.Data.URL, nil
}
func publicIP(ip net.IP) bool {
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	for _, reserved := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "64:ff9b::/96", "2001:db8::/32"} {
		_, block, _ := net.ParseCIDR(reserved)
		if block.Contains(ip) {
			return false
		}
	}
	return true
}
func downloadModel(ctx context.Context, raw string) ([]byte, error) {
	u, e := url.Parse(raw)
	if e != nil || u.Scheme != "https" || u.Hostname() != "assets.meshy.ai" || u.User != nil || (u.Port() != "" && u.Port() != "443") {
		return nil, errors.New("invalid model origin")
	}
	transport := &http.Transport{DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, e := net.SplitHostPort(address)
		if e != nil {
			return nil, e
		}
		ips, e := net.DefaultResolver.LookupIPAddr(ctx, host)
		if e != nil {
			return nil, e
		}
		for _, ip := range ips {
			if !publicIP(ip.IP) {
				return nil, errors.New("private model origin")
			}
		}
		for _, ip := range ips {
			conn, e := (&net.Dialer{Timeout: 15 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
			if e == nil {
				return conn, nil
			}
		}
		return nil, errors.New("model origin unavailable")
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 90 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if e != nil {
		return nil, e
	}
	res, e := client.Do(req)
	if e != nil {
		return nil, errors.New("model download failed")
	}
	defer res.Body.Close()
	if res.StatusCode != 200 || res.ContentLength > 32<<20 {
		return nil, errors.New("model response invalid " + strconv.Itoa(res.StatusCode))
	}
	data, e := io.ReadAll(io.LimitReader(res.Body, (32<<20)+1))
	if e != nil || len(data) > 32<<20 {
		return nil, errors.New("model too large")
	}
	return data, nil
}

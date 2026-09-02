package agent

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type ImageLoader struct {
	allowedHosts map[string]struct{}
	maxBytes     int64
	client       *http.Client
}

func NewImageLoader(allowedHosts []string, maxBytes int) *ImageLoader {
	hosts := make(map[string]struct{}, len(allowedHosts))
	for _, host := range allowedHosts {
		if normalized := strings.ToLower(strings.TrimSpace(host)); normalized != "" {
			hosts[normalized] = struct{}{}
		}
	}
	if maxBytes <= 0 {
		maxBytes = 8 << 20
	}
	loader := &ImageLoader{allowedHosts: hosts, maxBytes: int64(maxBytes)}
	loader.client = &http.Client{
		Timeout:       20 * time.Second,
		CheckRedirect: func(req *http.Request, _ []*http.Request) error { return loader.validateURL(req.URL) },
	}
	return loader
}

func (l *ImageLoader) Load(ctx context.Context, rawURLs []string) ([]string, error) {
	if len(rawURLs) > 3 {
		return nil, fmt.Errorf("at most 3 images are allowed")
	}
	result := make([]string, 0, len(rawURLs))
	for _, rawURL := range rawURLs {
		parsed, err := url.Parse(strings.TrimSpace(rawURL))
		if err != nil || parsed.Scheme == "" {
			return nil, fmt.Errorf("invalid image URL")
		}
		if err := l.validateURL(parsed); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
		if err != nil {
			return nil, err
		}
		resp, err := l.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("load image: %w", err)
		}
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			resp.Body.Close()
			return nil, fmt.Errorf("image server returned %d", resp.StatusCode)
		}
		data, readErr := io.ReadAll(io.LimitReader(resp.Body, l.maxBytes+1))
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read image: %w", readErr)
		}
		if int64(len(data)) > l.maxBytes {
			return nil, fmt.Errorf("image exceeds size limit")
		}
		contentType := http.DetectContentType(data)
		if !allowedImageType(contentType) {
			return nil, fmt.Errorf("unsupported image type")
		}
		result = append(result, "data:"+contentType+";base64,"+base64.StdEncoding.EncodeToString(data))
	}
	return result, nil
}

func (l *ImageLoader) validateURL(parsed *url.URL) error {
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("unsupported image URL scheme")
	}
	host := strings.ToLower(parsed.Hostname())
	if _, allowed := l.allowedHosts[host]; !allowed {
		return fmt.Errorf("image host is not allowed")
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast()) {
		return fmt.Errorf("unsafe image host")
	}
	return nil
}

func allowedImageType(value string) bool {
	switch strings.Split(value, ";")[0] {
	case "image/jpeg", "image/png", "image/gif", "image/webp":
		return true
	default:
		return false
	}
}

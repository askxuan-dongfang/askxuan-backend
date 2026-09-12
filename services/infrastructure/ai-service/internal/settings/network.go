package settings

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

// Administrator-configured endpoints must never reach internal services or cloud metadata.
func normalizeURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Port() != "" && u.Port() != "443") {
		return "", errors.New("接口地址必须为不含账号、查询参数的 HTTPS 公网地址，端口仅支持 443")
	}
	if ip, e := netip.ParseAddr(u.Hostname()); e == nil && !publicIP(ip) {
		return "", errors.New("接口地址不能指向内网或本机")
	}
	if strings.Contains(u.EscapedPath(), "%") || strings.Contains(u.Path, "..") || len(raw) > 512 {
		return "", errors.New("接口地址路径无效")
	}
	u.Host = strings.ToLower(u.Hostname())
	if strings.Contains(u.Host, ":") {
		u.Host = "[" + u.Host + "]"
	}
	u.Path = strings.TrimRight(u.Path, "/")
	return u.String(), nil
}

func publicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return false
	}
	// Shared, documentation, reserved and translation ranges are not provider endpoints.
	for _, raw := range []string{"0.0.0.0/8", "100.64.0.0/10", "192.0.0.0/24", "192.0.2.0/24", "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4", "2001:db8::/32", "64:ff9b::/96", "64:ff9b:1::/48", "2002::/16", "2001::/32"} {
		if netip.MustParsePrefix(raw).Contains(ip) {
			return false
		}
	}
	return true
}

func secureClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil || port != "443" {
			return nil, errors.New("不支持的接口端口")
		}
		addresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
		if err != nil || len(addresses) == 0 {
			return nil, errors.New("接口域名解析失败")
		}
		for _, ip := range addresses {
			if !publicIP(ip) {
				return nil, errors.New("接口域名指向受限网络")
			}
		}
		dialer := net.Dialer{Timeout: 8 * time.Second}
		// Dial the validated address, preventing a second DNS lookup / rebinding.
		for _, ip := range addresses {
			conn, e := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if e == nil {
				return conn, nil
			}
		}
		return nil, errors.New("接口连接失败")
	}
	return &http.Client{Transport: transport, Timeout: 60 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return errors.New("接口不允许跳转") }}
}

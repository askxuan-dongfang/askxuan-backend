// Package paychannel integrates merchant APIs. There is deliberately no mock
// implementation: test gateways are injected by tests, never by HTTP requests.
package paychannel

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

var ErrDisabled = errors.New("payment channel disabled")
var ErrVerification = errors.New("payment channel verification failed")
var ErrPending = errors.New("payment channel outcome pending reconciliation")

type Config struct {
	Enabled        bool   `json:",default=false"`
	AppID          string `json:",optional"`
	MerchantID     string `json:",optional"`
	PrivateKeyFile string `json:",optional"`
	PublicKeyFile  string `json:",optional"`
	PublicKeyID    string `json:",optional"`
	MerchantSerial string `json:",optional"`
	APIv3KeyFile   string `json:",optional"`
	NotifyURL      string `json:",optional"`
	ReturnURL      string `json:",optional"`
	Sandbox        bool   `json:",default=false"`
}
type Order struct {
	No    string
	Cents int64
	IP    string
}
type Result struct {
	No, TradeNo, State string
	Cents              int64
}
type Refund struct {
	No, TradeNo, RefundNo string
	Cents, Total          int64
}
type Gateway interface {
	Checkout(context.Context, Order) (string, error)
	Query(context.Context, string) (Result, error)
	Verify(*http.Request) (Result, error)
	Refund(context.Context, Refund) (string, error)
}

func New(name string, c Config) (Gateway, error) {
	if !c.Enabled {
		return nil, nil
	}
	u, e := url.Parse(c.NotifyURL)
	if e != nil || u.Scheme != "https" || u.Host == "" {
		return nil, errors.New("enabled payment channel requires an HTTPS notify URL")
	}
	if c.AppID == "" {
		return nil, errors.New("payment app ID missing")
	}
	private, e := privateKey(c.PrivateKeyFile)
	if e != nil {
		return nil, e
	}
	public, e := publicKey(c.PublicKeyFile)
	if e != nil {
		return nil, e
	}
	client := &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	switch name {
	case "wechat":
		if c.MerchantID == "" || c.MerchantSerial == "" || c.PublicKeyID == "" || c.Sandbox {
			return nil, errors.New("WeChat merchant ID, serial and platform public key ID required; sandbox unsupported")
		}
		key, e := os.ReadFile(c.APIv3KeyFile)
		if e != nil {
			return nil, errors.New("cannot read WeChat APIv3 key file")
		}
		key = []byte(strings.TrimSpace(string(key)))
		if len(key) != 32 {
			return nil, errors.New("WeChat APIv3 key must be 32 bytes")
		}
		return &wechat{c: c, private: private, public: public, key: key, client: client, base: "https://api.mch.weixin.qq.com"}, nil
	case "alipay":
		if c.MerchantID == "" {
			return nil, errors.New("Alipay seller ID missing")
		}
		u, e := url.Parse(c.ReturnURL)
		if e != nil || u.Scheme != "https" || u.Host == "" {
			return nil, errors.New("Alipay HTTPS return URL required")
		}
		endpoint := "https://openapi.alipay.com/gateway.do"
		if c.Sandbox {
			endpoint = "https://openapi-sandbox.dl.alipaydev.com/gateway.do"
		}
		return &alipay{c: c, private: private, public: public, client: client, endpoint: endpoint}, nil
	default:
		return nil, ErrDisabled
	}
}
func privateKey(path string) (*rsa.PrivateKey, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, errors.New("cannot read payment private key file")
	}
	p, _ := pem.Decode(b)
	if p == nil {
		return nil, errors.New("invalid payment private PEM")
	}
	k, e := x509.ParsePKCS8PrivateKey(p.Bytes)
	if e == nil {
		if r, ok := k.(*rsa.PrivateKey); ok && r.N.BitLen() >= 2048 {
			return r, nil
		}
	}
	r, e := x509.ParsePKCS1PrivateKey(p.Bytes)
	if e != nil || r.N.BitLen() < 2048 {
		return nil, errors.New("payment RSA key must be >=2048 bits")
	}
	return r, nil
}
func publicKey(path string) (*rsa.PublicKey, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, errors.New("cannot read payment public key file")
	}
	p, _ := pem.Decode(b)
	if p == nil {
		return nil, errors.New("invalid payment public PEM")
	}
	k, e := x509.ParsePKIXPublicKey(p.Bytes)
	if e == nil {
		if r, ok := k.(*rsa.PublicKey); ok && r.N.BitLen() >= 2048 {
			return r, nil
		}
	}
	return nil, errors.New("invalid payment RSA public key")
}
func sign(key *rsa.PrivateKey, b []byte) (string, error) {
	h := sha256.Sum256(b)
	s, e := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, h[:])
	return base64.StdEncoding.EncodeToString(s), e
}
func verify(key *rsa.PublicKey, b []byte, s string) error {
	sig, e := base64.StdEncoding.DecodeString(s)
	if e != nil {
		return ErrVerification
	}
	h := sha256.Sum256(b)
	if rsa.VerifyPKCS1v15(key, crypto.SHA256, h[:], sig) != nil {
		return ErrVerification
	}
	return nil
}
func canonical(v url.Values) string {
	keys := make([]string, 0, len(v))
	for k := range v {
		if k != "sign" && k != "sign_type" && v.Get(k) != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	parts := []string{}
	for _, k := range keys {
		parts = append(parts, k+"="+v.Get(k))
	}
	return strings.Join(parts, "&")
}
func readBody(r io.Reader) ([]byte, error) {
	b, e := io.ReadAll(io.LimitReader(r, 1<<20+1))
	if len(b) > 1<<20 {
		return nil, errors.New("payment response too large")
	}
	return b, e
}
func money(c int64) string { return fmt.Sprintf("%d.%02d", c/100, c%100) }

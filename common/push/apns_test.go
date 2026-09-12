package push

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"github.com/golang-jwt/jwt/v4"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestAPNSAuthorizationAndDelivery(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	status := 200
	reason := ""
	seen := false
	a := &APNS{key: key, keyID: "TESTKEY", teamID: "TESTTEAM"}
	a.client = &http.Client{Transport: transportFunc(func(r *http.Request) (*http.Response, error) {
		seen = true
		if r.URL.Host != "api.sandbox.push.apple.com" || r.Header.Get("apns-topic") != "com.dongfang.customer" || r.Header.Get("apns-push-type") != "alert" {
			t.Error("incorrect routing")
		}
		token, err := jwt.Parse(strings.TrimPrefix(r.Header.Get("authorization"), "bearer "), func(t *jwt.Token) (any, error) { return &key.PublicKey, nil })
		if err != nil || !token.Valid || token.Header["kid"] != "TESTKEY" {
			t.Error("invalid signed provider token")
		}
		if r.Header.Get("apns-expiration") == "" {
			t.Error("missing expiry")
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(`{"reason":"` + reason + `"}`)), Header: make(http.Header)}, nil
	})}
	payload := map[string]any{"aps": map[string]any{"alert": "测试通知"}}
	if permanent, err := a.SendUntil(context.Background(), "test-device", "com.dongfang.customer", "sandbox", "test", payload, time.Now().Add(time.Minute)); err != nil || permanent || !seen {
		t.Fatal("valid notification failed")
	}
	status = 410
	reason = "Unregistered"
	if permanent, err := a.Send(context.Background(), "test-device", "com.dongfang.customer", "sandbox", "test", payload); err == nil || !permanent {
		t.Fatal("invalid token not deactivated")
	}
	status = 503
	reason = "ServiceUnavailable"
	if permanent, err := a.Send(context.Background(), "test-device", "com.dongfang.customer", "sandbox", "test", payload); err == nil || permanent {
		t.Fatal("temporary outage discarded device")
	}
}

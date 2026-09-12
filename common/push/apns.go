// Package push delivers real APNs alerts. Credentials are read from a mounted
// private key, never from the repository or notification payload.
package push

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"github.com/golang-jwt/jwt/v4"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type APNS struct {
	key           *ecdsa.PrivateKey
	keyID, teamID string
	mu            sync.Mutex
	token         string
	expires       time.Time
	client        *http.Client
}

func NewAPNS(keyFile, keyID, teamID string) (*APNS, error) {
	if keyFile == "" || keyID == "" || teamID == "" {
		return nil, fmt.Errorf("APNs credentials not configured")
	}
	pem, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, err
	}
	key, err := jwt.ParseECPrivateKeyFromPEM(pem)
	if err != nil {
		return nil, err
	}
	return &APNS{key: key, keyID: keyID, teamID: teamID, client: &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{ForceAttemptHTTP2: true}}}, nil
}
func (a *APNS) authorization() (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if time.Now().Before(a.expires) {
		return a.token, nil
	}
	t := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{"iss": a.teamID, "iat": time.Now().Unix()})
	t.Header["kid"] = a.keyID
	s, err := t.SignedString(a.key)
	if err == nil {
		a.token = s
		a.expires = time.Now().Add(45 * time.Minute)
	}
	return s, err
}

// Send returns permanent=true only for invalid/unregistered device tokens.
func (a *APNS) Send(ctx context.Context, token, topic, environment, collapse string, payload any) (permanent bool, err error) {
	return a.SendUntil(ctx, token, topic, environment, collapse, payload, time.Now().Add(24*time.Hour))
}
func (a *APNS) SendUntil(ctx context.Context, token, topic, environment, collapse string, payload any, expires time.Time) (bool, error) {
	auth, err := a.authorization()
	if err != nil {
		return false, err
	}
	data, err := json.Marshal(payload)
	if err != nil || len(data) > 4096 {
		return false, fmt.Errorf("invalid APNs payload")
	}
	host := "https://api.push.apple.com"
	if environment == "sandbox" {
		host = "https://api.sandbox.push.apple.com"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, host+"/3/device/"+token, bytes.NewReader(data))
	if err != nil {
		return false, err
	}
	req.Header.Set("authorization", "bearer "+auth)
	req.Header.Set("apns-topic", topic)
	req.Header.Set("apns-push-type", "alert")
	req.Header.Set("apns-priority", "10")
	req.Header.Set("apns-collapse-id", collapse)
	req.Header.Set("apns-expiration", fmt.Sprint(expires.Unix()))
	resp, err := a.client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		return false, nil
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var failure struct {
		Reason string `json:"reason"`
	}
	_ = json.Unmarshal(body, &failure)
	return resp.StatusCode == 410 || failure.Reason == "BadDeviceToken" || failure.Reason == "DeviceTokenNotForTopic", fmt.Errorf("APNs status=%d reason=%s", resp.StatusCode, strings.TrimSpace(failure.Reason))
}

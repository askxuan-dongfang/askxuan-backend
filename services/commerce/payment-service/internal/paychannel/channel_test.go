package paychannel

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestDisabledRequiresNoCredentials(t *testing.T) {
	for _, n := range []string{"wechat", "alipay"} {
		g, e := New(n, Config{})
		if e != nil || g != nil {
			t.Fatal(n, e)
		}
		if _, e = New(n, Config{Enabled: true}); e == nil {
			t.Fatal("enabled channel accepted incomplete config")
		}
	}
}
func TestAlipaySignedNotification(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	a := alipay{c: Config{AppID: "app", MerchantID: "seller"}, public: &key.PublicKey}
	v := url.Values{"app_id": {"app"}, "seller_id": {"seller"}, "out_trade_no": {"W123"}, "trade_no": {"T456"}, "trade_status": {"TRADE_SUCCESS"}, "total_amount": {"19.90"}, "sign_type": {"RSA2"}}
	signature, _ := sign(key, []byte(canonical(v)))
	v.Set("sign", signature)
	verifyRequest := func() (Result, error) {
		r := httptest.NewRequest("POST", "/", strings.NewReader(v.Encode()))
		return a.Verify(r)
	}
	result, e := verifyRequest()
	if e != nil || result.Cents != 1990 || result.No != "W123" {
		t.Fatal(result, e)
	}
	v.Set("total_amount", "1990.00")
	if _, e = verifyRequest(); e == nil {
		t.Fatal("tampered amount accepted")
	}
	v.Set("total_amount", "19.90")
	v.Set("seller_id", "another")
	if _, e = verifyRequest(); e == nil {
		t.Fatal("foreign merchant accepted")
	}
	v.Set("seller_id", "seller")
	v.Add("total_amount", "19.90")
	if _, e = verifyRequest(); e == nil {
		t.Fatal("duplicate form parameters accepted")
	}
}
func TestWechatResponseSignature(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	w := wechat{c: Config{PublicKeyID: "KEY_ID"}, public: &key.PublicKey}
	b := []byte(`{"trade_state":"SUCCESS"}`)
	r := httptest.NewRequest("POST", "/", nil)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	r.Header.Set("Wechatpay-Timestamp", ts)
	r.Header.Set("Wechatpay-Nonce", "nonce")
	r.Header.Set("Wechatpay-Serial", "KEY_ID")
	s, _ := sign(key, []byte(ts+"\nnonce\n"+string(b)+"\n"))
	r.Header.Set("Wechatpay-Signature", s)
	if e := w.verified(r.Header, b); e != nil {
		t.Fatal(e)
	}
	if e := w.verified(r.Header, append(b, ' ')); e == nil {
		t.Fatal("tampered response accepted")
	}
	r.Header.Set("Wechatpay-Serial", "other")
	if e := w.verified(r.Header, b); e == nil {
		t.Fatal("untrusted key accepted")
	}
	r.Header.Set("Wechatpay-Serial", "KEY_ID")
	ts = strconv.FormatInt(time.Now().Add(-6*time.Minute).Unix(), 10)
	r.Header.Set("Wechatpay-Timestamp", ts)
	s, _ = sign(key, []byte(ts+"\nnonce\n"+string(b)+"\n"))
	r.Header.Set("Wechatpay-Signature", s)
	if e := w.verified(r.Header, b); e == nil {
		t.Fatal("stale signed response accepted")
	}
}
func TestAlipayRequestSigningIncludesSignType(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	a := alipay{private: key}
	v := url.Values{"app_id": {"app"}, "sign_type": {"RSA2"}, "biz_content": {`{"subject":"a&b=c"}`}}
	signed, e := a.signed(v)
	if e != nil {
		t.Fatal(e)
	}
	message := `app_id=app&biz_content={"subject":"a&b=c"}&sign_type=RSA2`
	if e = verify(&key.PublicKey, []byte(message), signed.Get("sign")); e != nil {
		t.Fatal(e)
	}
	var p any
	if json.Unmarshal([]byte(v.Get("biz_content")), &p) != nil {
		t.Fatal("invalid business payload")
	}
}

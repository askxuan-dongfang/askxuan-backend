package paychannel

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type wechat struct {
	c       Config
	private *rsa.PrivateKey
	public  *rsa.PublicKey
	key     []byte
	client  *http.Client
	base    string
}

func (w *wechat) verified(h http.Header, b []byte) error {
	if h.Get("Wechatpay-Serial") != w.c.PublicKeyID {
		return ErrVerification
	}
	ts := h.Get("Wechatpay-Timestamp")
	n, e := strconv.ParseInt(ts, 10, 64)
	if e != nil || time.Now().Unix()-n > 300 || n-time.Now().Unix() > 300 || h.Get("Wechatpay-Nonce") == "" {
		return ErrVerification
	}
	return verify(w.public, []byte(ts+"\n"+h.Get("Wechatpay-Nonce")+"\n"+string(b)+"\n"), h.Get("Wechatpay-Signature"))
}
func (w *wechat) call(ctx context.Context, method, path string, body any, out any) error {
	var b []byte
	var e error
	if body != nil {
		b, e = json.Marshal(body)
		if e != nil {
			return e
		}
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := uuid.NewString()
	s, e := sign(w.private, []byte(method+"\n"+path+"\n"+ts+"\n"+nonce+"\n"+string(b)+"\n"))
	if e != nil {
		return e
	}
	req, e := http.NewRequestWithContext(ctx, method, w.base+path, bytes.NewReader(b))
	if e != nil {
		return e
	}
	req.Header.Set("Authorization", fmt.Sprintf(`WECHATPAY2-SHA256-RSA2048 mchid="%s",nonce_str="%s",timestamp="%s",serial_no="%s",signature="%s"`, w.c.MerchantID, nonce, ts, w.c.MerchantSerial, s))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Wechatpay-Serial", w.c.PublicKeyID)
	r, e := w.client.Do(req)
	if e != nil {
		return ErrPending
	}
	defer r.Body.Close()
	b, e = readBody(r.Body)
	if e != nil {
		return e
	}
	if e = w.verified(r.Header, b); e != nil {
		return e
	}
	if r.StatusCode < 200 || r.StatusCode >= 300 {
		return ErrPending
	}
	return json.Unmarshal(b, out)
}
func (w *wechat) Checkout(ctx context.Context, o Order) (string, error) {
	if net.ParseIP(o.IP) == nil {
		return "", fmt.Errorf("payer IP unavailable")
	}
	var out struct {
		URL string `json:"h5_url"`
	}
	e := w.call(ctx, "POST", "/v3/pay/transactions/h5", map[string]any{"appid": w.c.AppID, "mchid": w.c.MerchantID, "description": "问玄东方钱包充值", "out_trade_no": o.No, "notify_url": w.c.NotifyURL, "time_expire": time.Now().Add(15 * time.Minute).Format(time.RFC3339), "amount": map[string]any{"total": o.Cents, "currency": "CNY"}, "scene_info": map[string]any{"payer_client_ip": o.IP, "h5_info": map[string]any{"type": "Wap"}}}, &out)
	u, err := url.Parse(out.URL)
	if e == nil && (err != nil || u.Scheme != "https" || u.Host != "wx.tenpay.com") {
		return "", ErrVerification
	}
	return out.URL, e
}

type wxPayment struct {
	AppID    string `json:"appid"`
	Merchant string `json:"mchid"`
	No       string `json:"out_trade_no"`
	Trade    string `json:"transaction_id"`
	State    string `json:"trade_state"`
	Amount   struct {
		Total    int64  `json:"total"`
		Currency string `json:"currency"`
	} `json:"amount"`
}

func (w *wechat) result(p wxPayment) (Result, error) {
	if p.AppID != w.c.AppID || p.Merchant != w.c.MerchantID || p.No == "" || p.Amount.Currency != "CNY" || p.Amount.Total <= 0 {
		return Result{}, ErrVerification
	}
	state := "pending"
	if p.State == "SUCCESS" {
		state = "success"
		if p.Trade == "" {
			return Result{}, ErrVerification
		}
	} else if p.State == "CLOSED" || p.State == "REVOKED" {
		state = "closed"
	}
	return Result{No: p.No, TradeNo: p.Trade, State: state, Cents: p.Amount.Total}, nil
}
func (w *wechat) Query(ctx context.Context, no string) (Result, error) {
	var p wxPayment
	e := w.call(ctx, "GET", "/v3/pay/transactions/out-trade-no/"+url.PathEscape(no)+"?mchid="+url.QueryEscape(w.c.MerchantID), nil, &p)
	if e != nil {
		return Result{}, e
	}
	return w.result(p)
}
func (w *wechat) Verify(r *http.Request) (Result, error) {
	b, e := readBody(r.Body)
	if e != nil {
		return Result{}, e
	}
	if e = w.verified(r.Header, b); e != nil {
		return Result{}, e
	}
	var n struct {
		Type     string `json:"event_type"`
		Resource struct {
			Algorithm  string `json:"algorithm"`
			Ciphertext string `json:"ciphertext"`
			Nonce      string `json:"nonce"`
			AD         string `json:"associated_data"`
		} `json:"resource"`
	}
	if json.Unmarshal(b, &n) != nil || n.Type != "TRANSACTION.SUCCESS" || n.Resource.Algorithm != "AEAD_AES_256_GCM" {
		return Result{}, ErrVerification
	}
	block, e := aes.NewCipher(w.key)
	if e != nil {
		return Result{}, e
	}
	g, e := cipher.NewGCM(block)
	if e != nil {
		return Result{}, e
	}
	c, e := base64.StdEncoding.DecodeString(n.Resource.Ciphertext)
	if e != nil || len(n.Resource.Nonce) != g.NonceSize() {
		return Result{}, ErrVerification
	}
	plain, e := g.Open(nil, []byte(n.Resource.Nonce), c, []byte(n.Resource.AD))
	if e != nil {
		return Result{}, ErrVerification
	}
	var p wxPayment
	if json.Unmarshal(plain, &p) != nil {
		return Result{}, ErrVerification
	}
	return w.result(p)
}
func (w *wechat) Refund(ctx context.Context, f Refund) (string, error) {
	var out struct {
		No     string `json:"out_refund_no"`
		Trade  string `json:"transaction_id"`
		Status string `json:"status"`
		Amount struct {
			Refund   int64  `json:"refund"`
			Total    int64  `json:"total"`
			Currency string `json:"currency"`
		} `json:"amount"`
	}
	// The stable out_refund_no makes a retry after an ambiguous network result safe.
	e := w.call(ctx, "POST", "/v3/refund/domestic/refunds", map[string]any{"transaction_id": f.TradeNo, "out_refund_no": f.RefundNo, "reason": "钱包充值原路退回", "amount": map[string]any{"refund": f.Cents, "total": f.Total, "currency": "CNY"}}, &out)
	if e != nil {
		e = w.call(ctx, "GET", "/v3/refund/domestic/refunds/"+url.PathEscape(f.RefundNo), nil, &out)
	}
	if e != nil {
		return "pending", e
	}
	if out.No != f.RefundNo || out.Trade != f.TradeNo || out.Amount.Refund != f.Cents || out.Amount.Total != f.Total || out.Amount.Currency != "CNY" {
		return "pending", ErrVerification
	}
	if out.Status == "SUCCESS" {
		return "success", nil
	}
	return "pending", nil
}

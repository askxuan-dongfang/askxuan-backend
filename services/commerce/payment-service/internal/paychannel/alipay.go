package paychannel

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"github.com/askxuan/payment-service/internal/balance"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type alipay struct {
	c        Config
	private  *rsa.PrivateKey
	public   *rsa.PublicKey
	client   *http.Client
	endpoint string
}

func (a *alipay) params(method string, biz any) (url.Values, error) {
	b, e := json.Marshal(biz)
	if e != nil {
		return nil, e
	}
	v := url.Values{"app_id": {a.c.AppID}, "method": {method}, "charset": {"utf-8"}, "sign_type": {"RSA2"}, "timestamp": {time.Now().In(time.FixedZone("CST", 8*3600)).Format("2006-01-02 15:04:05")}, "version": {"1.0"}, "biz_content": {string(b)}}
	return v, nil
}
func (a *alipay) signed(v url.Values) (url.Values, error) {
	// API signing includes sign_type; notification verification excludes it.
	var parts []string
	// Do not split/reorder values that may themselves contain '&'.
	keys := []string{}
	for k := range v {
		if k != "sign" && v.Get(k) != "" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		parts = append(parts, k+"="+v.Get(k))
	}
	s, e := sign(a.private, []byte(strings.Join(parts, "&")))
	v.Set("sign", s)
	return v, e
}
func (a *alipay) Checkout(ctx context.Context, o Order) (string, error) {
	v, e := a.params("alipay.trade.wap.pay", map[string]any{"out_trade_no": o.No, "total_amount": money(o.Cents), "subject": "问玄东方钱包充值", "product_code": "QUICK_WAP_WAY", "timeout_express": "15m", "seller_id": a.c.MerchantID})
	if e != nil {
		return "", e
	}
	v.Set("notify_url", a.c.NotifyURL)
	v.Set("return_url", a.c.ReturnURL)
	v, e = a.signed(v)
	return a.endpoint + "?" + v.Encode(), e
}
func (a *alipay) call(ctx context.Context, method string, biz any, out any) error {
	v, e := a.params(method, biz)
	if e != nil {
		return e
	}
	v, e = a.signed(v)
	if e != nil {
		return e
	}
	req, e := http.NewRequestWithContext(ctx, "POST", a.endpoint, strings.NewReader(v.Encode()))
	if e != nil {
		return e
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r, e := a.client.Do(req)
	if e != nil {
		return ErrPending
	}
	defer r.Body.Close()
	b, e := readBody(r.Body)
	if e != nil {
		return e
	}
	if r.StatusCode != 200 {
		return ErrPending
	}
	var envelope map[string]json.RawMessage
	if json.Unmarshal(b, &envelope) != nil {
		return ErrVerification
	}
	raw := envelope[strings.ReplaceAll(method, ".", "_")+"_response"]
	var signature string
	if json.Unmarshal(envelope["sign"], &signature) != nil || len(raw) == 0 {
		return ErrVerification
	}
	if e = verify(a.public, raw, signature); e != nil {
		return e
	}
	var code struct {
		Code string `json:"code"`
	}
	if json.Unmarshal(raw, &code) != nil || code.Code != "10000" {
		return ErrPending
	}
	return json.Unmarshal(raw, out)
}
func (a *alipay) Query(ctx context.Context, no string) (Result, error) {
	var p struct {
		No     string `json:"out_trade_no"`
		Trade  string `json:"trade_no"`
		Status string `json:"trade_status"`
		Total  string `json:"total_amount"`
	}
	e := a.call(ctx, "alipay.trade.query", map[string]any{"out_trade_no": no}, &p)
	if e != nil {
		return Result{}, e
	}
	if p.No != no {
		return Result{}, ErrVerification
	}
	c, e := balance.ParseCents(p.Total)
	if e != nil {
		return Result{}, ErrVerification
	}
	state := "pending"
	if p.Status == "TRADE_SUCCESS" || p.Status == "TRADE_FINISHED" {
		state = "success"
		if p.Trade == "" {
			return Result{}, ErrVerification
		}
	} else if p.Status == "TRADE_CLOSED" {
		state = "closed"
	}
	return Result{No: no, TradeNo: p.Trade, Cents: c, State: state}, nil
}
func (a *alipay) Verify(r *http.Request) (Result, error) {
	b, e := readBody(r.Body)
	if e != nil {
		return Result{}, e
	}
	v, e := url.ParseQuery(string(b))
	if e != nil {
		return Result{}, ErrVerification
	}
	for _, values := range v {
		if len(values) != 1 {
			return Result{}, ErrVerification
		}
	}
	if v.Get("sign_type") != "RSA2" || v.Get("app_id") != a.c.AppID || v.Get("seller_id") != a.c.MerchantID {
		return Result{}, ErrVerification
	}
	if e = verify(a.public, []byte(canonical(v)), v.Get("sign")); e != nil {
		return Result{}, e
	}
	if v.Get("trade_status") != "TRADE_SUCCESS" && v.Get("trade_status") != "TRADE_FINISHED" {
		return Result{}, ErrVerification
	}
	c, e := balance.ParseCents(v.Get("total_amount"))
	if e != nil || v.Get("out_trade_no") == "" || v.Get("trade_no") == "" {
		return Result{}, ErrVerification
	}
	return Result{No: v.Get("out_trade_no"), TradeNo: v.Get("trade_no"), State: "success", Cents: c}, nil
}
func (a *alipay) Refund(ctx context.Context, f Refund) (string, error) {
	var p struct {
		No    string `json:"out_trade_no"`
		Trade string `json:"trade_no"`
		Fee   string `json:"refund_fee"`
	}
	e := a.call(ctx, "alipay.trade.refund", map[string]any{"out_trade_no": f.No, "trade_no": f.TradeNo, "out_request_no": f.RefundNo, "refund_amount": money(f.Cents), "refund_reason": "钱包充值原路退回"}, &p)
	if e != nil {
		return "pending", e
	}
	c, e := balance.ParseCents(p.Fee)
	if e != nil || c != f.Cents || p.No != f.No || p.Trade != f.TradeNo {
		return "pending", ErrVerification
	}
	return "success", nil
}

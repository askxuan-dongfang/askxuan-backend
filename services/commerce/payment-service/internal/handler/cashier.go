package handler

import (
	"encoding/json"
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/payment-service/internal/balance"
	"github.com/askxuan/payment-service/internal/cashier"
	"github.com/askxuan/payment-service/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	"net"
	"net/http"
	"strconv"
)

func cashError(w http.ResponseWriter, e error) {
	if _, ok := e.(*common.BizError); !ok {
		logx.Error("cashier request failed")
		e = common.ErrSystem
	}
	common.JsonError(w, e)
}
func registerCashier(server *rest.Server, s *svc.ServiceContext) {
	auth := &middleware.AuthConfig{Secret: s.Config.Auth.AccessSecret}
	role := &middleware.AdminAuthConfig{AllowedRoles: []string{"customer"}}
	add := func(method, path string, fn http.HandlerFunc) {
		server.AddRoute(rest.Route{Method: method, Path: "/api/v1/payments/" + path, Handler: auth.AuthFunc(role.AdminAuthFunc(fn))})
	}
	user := func(r *http.Request) string { return strconv.FormatInt(middleware.UserIDFromCtx(r.Context()), 10) }
	decode := func(w http.ResponseWriter, r *http.Request, v any) bool {
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(v) != nil {
			common.JsonError(w, common.ErrParam)
			return false
		}
		return true
	}
	add("GET", "checkout", func(w http.ResponseWriter, r *http.Request) {
		q, e := s.Cashier.Quote(r.Context(), user(r), cashier.OrderRequest{OrderType: r.URL.Query().Get("orderType"), OrderNo: r.URL.Query().Get("orderNo")})
		if e != nil {
			cashError(w, e)
			return
		}
		common.Ok(w, q)
	})
	add("POST", "checkout", func(w http.ResponseWriter, r *http.Request) {
		var req cashier.OrderRequest
		if !decode(w, r, &req) {
			return
		}
		p, e := s.Cashier.Pay(r.Context(), user(r), req)
		if e != nil {
			cashError(w, e)
			return
		}
		common.Ok(w, p)
	})
	add("GET", "wallet/balance", func(w http.ResponseWriter, r *http.Request) {
		a, e := balance.Read(r.Context(), s.DB, user(r))
		if e != nil {
			cashError(w, e)
			return
		}
		entries := []balance.Entry{}
		page := 1
		if raw := r.URL.Query().Get("page"); raw != "" {
			page, e = strconv.Atoi(raw)
			if e != nil || page < 1 || page > 100000 {
				common.JsonError(w, common.ErrParam)
				return
			}
		}
		e = s.DB.QueryRowsCtx(r.Context(), &entries, `SELECT id,kind,reference_no,available_delta,held_delta,available_after,DATE_FORMAT(created_at,'%Y-%m-%d %H:%i:%s') created_at FROM wallet_ledger WHERE user_id=? ORDER BY id DESC LIMIT 30 OFFSET ?`, user(r), (page-1)*30)
		if e != nil {
			cashError(w, e)
			return
		}
		recharges := []cashier.Recharge{}
		e = s.DB.QueryRowsCtx(r.Context(), &recharges, `SELECT recharge_no,user_id,request_id,channel,amount_cents,status,COALESCE(trade_no,'') trade_no,'' pay_url,refund_no,refund_cents,DATE_FORMAT(created_at,'%Y-%m-%d %H:%i:%s') created_at FROM wallet_recharge WHERE user_id=? ORDER BY created_at DESC LIMIT 30`, user(r))
		if e != nil {
			cashError(w, e)
			return
		}
		channels := []map[string]any{}
		for _, name := range []string{"wechat", "alipay"} {
			channels = append(channels, map[string]any{"id": name, "enabled": s.Cashier.Enabled && s.Cashier.Channels[name] != nil})
		}
		common.Ok(w, map[string]any{"availableCents": a.Available, "heldCents": a.Held, "enabled": s.Cashier.Enabled, "channels": channels, "entries": entries, "recharges": recharges, "page": page, "hasMore": len(entries) == 30})
	})
	add("POST", "wallet/recharges", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Channel     string `json:"channel"`
			RequestID   string `json:"requestId"`
			AmountCents int64  `json:"amountCents"`
		}
		if !decode(w, r, &req) {
			return
		}
		ip, _, _ := net.SplitHostPort(r.RemoteAddr)
		// Payment service is internal; ingress must overwrite X-Real-IP.
		if parsed := net.ParseIP(r.Header.Get("X-Real-IP")); parsed != nil {
			ip = parsed.String()
		}
		f, e := s.Cashier.CreateRecharge(r.Context(), user(r), req.Channel, req.RequestID, ip, req.AmountCents)
		if e != nil {
			cashError(w, e)
			return
		}
		common.Ok(w, f)
	})
	add("GET", "wallet/recharge", func(w http.ResponseWriter, r *http.Request) {
		f, e := s.Cashier.Get(r.Context(), user(r), r.URL.Query().Get("no"))
		if e != nil {
			cashError(w, e)
			return
		}
		common.Ok(w, f)
	})
	add("POST", "wallet/recharge/refund", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			RechargeNo  string `json:"rechargeNo"`
			AmountCents int64  `json:"amountCents"`
		}
		if !decode(w, r, &req) {
			return
		}
		f, e := s.Cashier.RefundRecharge(r.Context(), user(r), req.RechargeNo, req.AmountCents)
		if e != nil {
			cashError(w, e)
			return
		}
		common.Ok(w, f)
	})
}
func rechargeCallback(s *svc.ServiceContext, channel string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		g := s.Cashier.Channels[channel]
		if !s.Cashier.Enabled || g == nil {
			http.Error(w, "channel disabled", 503)
			return
		}
		result, e := g.Verify(r)
		if e != nil {
			http.Error(w, "invalid notification", 400)
			return
		}
		if e = s.Cashier.Apply(r.Context(), channel, result); e != nil {
			http.Error(w, "retry later", 503)
			return
		}
		if channel == "alipay" {
			w.Write([]byte("success"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"code":"SUCCESS","message":"OK"}`))
	}
}

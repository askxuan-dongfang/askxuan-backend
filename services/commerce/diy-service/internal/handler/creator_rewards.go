package handler

import (
	"fmt"
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/diy-service/internal/model"
	"github.com/askxuan/diy-service/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/rest"
	"github.com/zeromicro/go-zero/rest/httpx"
	"net/http"
	"strconv"
)

func registerCreatorRewards(server *rest.Server, s *svc.ServiceContext) {
	auth := &middleware.AuthConfig{Secret: s.Config.AuthSecret}
	customer := &middleware.AdminAuthConfig{AllowedRoles: []string{"customer"}}
	admin := &middleware.AdminAuthConfig{AllowedRoles: []string{"platform_super"}}
	for _, r := range []rest.Route{
		{Method: "GET", Path: "/api/v1/diy/creator-wallet", Handler: creatorWallet(s, false)},
		{Method: "POST", Path: "/api/v1/diy/creator-withdrawals", Handler: creatorApply(s)},
	} {
		r.Handler = auth.AuthFunc(customer.AdminAuthFunc(r.Handler))
		server.AddRoute(r)
	}
	for _, r := range []rest.Route{
		{Method: "GET", Path: "/api/v1/admin/diy/creator-wallet", Handler: creatorWallet(s, true)},
		{Method: "GET", Path: "/api/v1/admin/diy/reward-policy", Handler: rewardPolicy(s, false)},
		{Method: "PUT", Path: "/api/v1/admin/diy/reward-policy", Handler: rewardPolicy(s, true)},
		{Method: "POST", Path: "/api/v1/admin/diy/creator-withdrawals/:id/action", Handler: creatorReview(s)},
		{Method: "POST", Path: "/api/v1/admin/diy/rewards/settle", Handler: func(w http.ResponseWriter, r *http.Request) {
			e := model.ProcessRewards(r.Context(), s.DB)
			rewardReply(w, nil, e)
		}},
	} {
		r.Handler = auth.AuthFunc(admin.AdminAuthFunc(r.Handler))
		server.AddRoute(r)
	}
}
func rewardReply(w http.ResponseWriter, data interface{}, err error) {
	if err != nil {
		logx.Errorf("creator reward operation: %v", err)
		common.JsonError(w, err)
		return
	}
	common.Ok(w, data)
}
func rewardPolicy(s *svc.ServiceContext, write bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if write {
			var p model.RewardPolicy
			if httpx.Parse(r, &p) != nil {
				common.JsonError(w, common.ErrParam)
				return
			}
			if e := model.SaveRewardPolicy(r.Context(), s.DB, p, fmt.Sprint(middleware.UserIDFromCtx(r.Context()))); e != nil {
				rewardReply(w, nil, e)
				return
			}
		}
		p, e := model.ReadRewardPolicy(r.Context(), s.DB)
		rewardReply(w, p, e)
	}
}
func creatorApply(s *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input struct {
			Amount  int64  `json:"amountCents"`
			Key     string `json:"requestKey"`
			Holder  string `json:"holderName"`
			Bank    string `json:"bankName"`
			Account string `json:"accountNumber"`
		}
		if httpx.Parse(r, &input) != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		req := model.CreatorWithdrawal{Amount: input.Amount, Key: input.Key, Holder: input.Holder, Bank: input.Bank, Account: input.Account}
		id, e := model.ApplyCreatorWithdrawal(r.Context(), s.DB, fmt.Sprint(middleware.UserIDFromCtx(r.Context())), req, s.Config.AuthSecret)
		rewardReply(w, map[string]interface{}{"id": id}, e)
	}
}
func creatorReview(s *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID        int64  `path:"id"`
			Action    string `json:"action"`
			Reference string `json:"reference,optional"`
			Note      string `json:"note,optional"`
		}
		if httpx.Parse(r, &req) != nil {
			common.JsonError(w, common.ErrParam)
			return
		}
		e := model.ReviewCreatorWithdrawal(r.Context(), s.DB, req.ID, req.Action, req.Reference, req.Note, fmt.Sprint(middleware.UserIDFromCtx(r.Context())))
		rewardReply(w, map[string]bool{"updated": e == nil}, e)
	}
}
func creatorWallet(s *svc.ServiceContext, admin bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user := fmt.Sprint(middleware.UserIDFromCtx(r.Context()))
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		if page < 1 {
			page = 1
		}
		if page > 100000 {
			common.JsonError(w, common.ErrParam)
			return
		}
		where := " WHERE creator_id=?"
		withdrawWhere := " WHERE user_id=?"
		args := []interface{}{user}
		if admin {
			where = ""
			withdrawWhere = ""
			args = nil
		}
		ctx := r.Context()
		var a model.CreatorAccount
		var pending, total, withdrawTotal int64
		if !admin {
			if e := s.DB.QueryRowCtx(ctx, &a, "SELECT COALESCE(SUM(available_cents),0) available_cents,COALESCE(SUM(held_cents),0) held_cents,COALESCE(SUM(paid_cents),0) paid_cents FROM creator_cash_account WHERE user_id=?", user); e != nil {
				rewardReply(w, nil, e)
				return
			}
			if e := s.DB.QueryRowCtx(ctx, &pending, "SELECT COALESCE(SUM(r.amount_cents),0) FROM creator_reward r JOIN diy_order o ON o.id=r.order_id WHERE r.creator_id=? AND r.status='pending' AND r.kind='cash' AND o.payment_status='success' AND EXISTS(SELECT 1 FROM askxuan_payment.payment p WHERE p.order_no=o.order_no AND p.order_type='diy_order' AND p.status='success' AND p.channel IN ('wechat','alipay','balance'))", user); e != nil {
				rewardReply(w, nil, e)
				return
			}
		}
		if e := s.DB.QueryRowCtx(ctx, &total, "SELECT COUNT(*) FROM creator_reward"+where, args...); e != nil {
			rewardReply(w, nil, e)
			return
		}
		if e := s.DB.QueryRowCtx(ctx, &withdrawTotal, "SELECT COUNT(*) FROM creator_withdrawal"+withdrawWhere, args...); e != nil {
			rewardReply(w, nil, e)
			return
		}
		// Kept local to model-facing queries; all pagination inputs are bound.
		var rewards []model.CreatorReward
		var withdrawals []model.CreatorWithdrawal
		paged := append(append([]interface{}{}, args...), (page-1)*20)
		e := s.DB.QueryRowsCtx(ctx, &rewards, "SELECT id,order_id,order_no,design_id,creator_id,kind,base_cents,amount_cents,status,reason,wait_days,coupon_id,DATE_FORMAT(created_at,'%Y-%m-%d %H:%i:%s') created_at FROM creator_reward"+where+" ORDER BY id DESC LIMIT 20 OFFSET ?", paged...)
		if e != nil {
			rewardReply(w, nil, e)
			return
		}
		e = s.DB.QueryRowsCtx(ctx, &withdrawals, "SELECT id,user_id,request_key,amount_cents,holder_name,bank_name,account_number,status,COALESCE(transfer_reference,'') transfer_reference,note,DATE_FORMAT(created_at,'%Y-%m-%d %H:%i:%s') created_at FROM creator_withdrawal"+withdrawWhere+" ORDER BY id DESC LIMIT 20 OFFSET ?", paged...)
		if e != nil {
			rewardReply(w, nil, e)
			return
		}
		for i := range withdrawals {
			decoded, err := model.OpenPayout(s.Config.AuthSecret, withdrawals[i].User, withdrawals[i].Account)
			if err != nil {
				rewardReply(w, nil, common.ErrSystem)
				return
			}
			withdrawals[i].Account = decoded
		}
		if !admin {
			for i := range withdrawals {
				v := withdrawals[i].Account
				if len(v) > 4 {
					withdrawals[i].Account = "****" + v[len(v)-4:]
				}
			}
		}
		if rewards == nil {
			rewards = []model.CreatorReward{}
		}
		if withdrawals == nil {
			withdrawals = []model.CreatorWithdrawal{}
		}
		p, e := model.ReadRewardPolicy(ctx, s.DB)
		if e != nil {
			rewardReply(w, nil, e)
			return
		}
		common.Ok(w, map[string]interface{}{"account": a, "pendingCents": pending, "rewards": rewards, "withdrawals": withdrawals, "total": total, "withdrawalTotal": withdrawTotal, "page": page, "policy": p, "payoutMode": "manual_bank_transfer"})
	}
}

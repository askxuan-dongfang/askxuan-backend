// Package adminpolicy defines the finite, delegatable platform API surface.
// Provider roles retain their existing rules; unknown routes never inherit permission.
package adminpolicy

import "strings"

type Capability struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	Resource   string `json:"resource"`
	Action     string `json:"action"`
	Assignable bool   `json:"assignable"`
}

var Catalog = []Capability{
	{"super.refund", "退款执行（仅超级管理员）", "refund", "super", false},
	{"super.sandbox", "模拟打款测试（仅超级管理员）", "sandbox", "super", false},
	{"platform.temples.read", "寺院档案·查看", "temples", "read", true},
	{"platform.temples.write", "寺院运营状态·管理", "temples", "write", true},
	{"platform.masters.read", "师傅档案·查看", "masters", "read", true},
	{"platform.masters.write", "师傅运营状态、咨询与服务·管理", "masters", "write", true},
	{"platform.users.read", "信众管理·查看", "users", "read", true},
	{"platform.users.write", "信众管理·管理", "users", "write", true},
	{"platform.products.read", "商品管理·查看", "products", "read", true},
	{"platform.products.write", "商品管理·管理", "products", "write", true},
	{"platform.orders.read", "商城订单·查看", "orders", "read", true},
	{"platform.orders.write", "商城订单·管理", "orders", "write", true},
	{"platform.diy.read", "DIY材料与订单·查看", "diy", "read", true},
	{"platform.diy.write", "DIY材料与订单·管理", "diy", "write", true},
	{"platform.points.read", "积分商城·查看", "points", "read", true},
	{"platform.points.write", "积分商城·管理", "points", "write", true},
	{"platform.logistics.read", "物流管理·查看", "logistics", "read", true},
	{"platform.logistics.write", "物流管理·管理", "logistics", "write", true},
	{"platform.finance.read", "财务审核与汇总·查看", "finance", "read", true},
	{"platform.finance.write", "财务审核与汇总·管理", "finance", "write", true},
	{"platform.marketing.read", "营销活动·查看", "marketing", "read", true},
	{"platform.marketing.write", "营销活动·管理", "marketing", "write", true},
	{"platform.moderation.read", "举报与敏感词·查看", "moderation", "read", true},
	{"platform.moderation.write", "举报与敏感词·管理", "moderation", "write", true},
	{"platform.operations.read", "操作日志·查看", "operations", "read", true},
	{"super.security", "账号与角色权限（仅超级管理员）", "security", "super", false},
	{"super.standards", "标准目录与首页展示（仅超级管理员）", "standards", "super", false},
	{"super.ai", "AI配置与密钥（仅超级管理员）", "ai", "super", false},
	{"super.onboarding", "入驻审核（仅超级管理员）", "onboarding", "super", false},
	{"super.service-overview", "全平台服务聚合（仅超级管理员）", "service-overview", "super", false},
	{"super.growth", "成长体系（仅超级管理员）", "growth", "super", false},
	{"super.creator", "创作奖励与提现（仅超级管理员）", "creator", "super", false},
	{"super.backup", "数据备份（仅超级管理员）", "backup", "super", false},
	{"super.community", "短视频内容（仅超级管理员）", "community", "super", false},
}

type Route struct{ Method, Template, Permission string }

// This static allowlist is intentionally reviewed with API additions; a prefix alone never grants new APIs.
var Routes = []Route{
	{"GET", "/api/v1/admin/auth/roles/:id/permissions", "super.security"},
	{"PUT", "/api/v1/admin/auth/roles/:id/permissions", "super.security"},
	{"PUT", "/api/v1/admin/orders/returns/:id/receive", "platform.orders.write"},
	{"GET", "/api/v1/admin/finance/payout-sandbox", "super.sandbox"},
	{"POST", "/api/v1/admin/finance/payout-sandbox", "super.sandbox"},
	{"GET", "/api/v1/admin/finance/payout-sandbox/:id", "super.sandbox"},
	{"POST", "/api/v1/admin/finance/payout-sandbox/:id/actions", "super.sandbox"},
	{"GET", "/api/v1/admin/points/products", "platform.points.read"},
	{"GET", "/api/v1/admin/points/orders", "platform.points.read"},
	{"POST", "/api/v1/admin/points/orders/:id/cancel", "platform.points.write"},
	{"POST", "/api/v1/admin/points/products", "platform.points.write"},
	{"PUT", "/api/v1/admin/points/products/:id", "platform.points.write"},
	{"POST", "/api/v1/admin/points/orders/:id/ship", "platform.points.write"},
	{"GET", "/api/v1/admin/points/report", "platform.points.read"},

	{"DELETE", "/api/v1/admin/audit/sensitive-words/:id", "platform.moderation.write"},
	{"DELETE", "/api/v1/admin/diy/blessing-services/:id", "platform.diy.write"},
	{"DELETE", "/api/v1/admin/products/:id", "platform.products.write"},
	{"DELETE", "/api/v1/admin/products/categories/:id", "platform.products.write"},
	{"DELETE", "/api/v1/admin/temples/images/:id", ""},
	{"GET", "/api/v1/admin/audit/operations", "platform.operations.read"},
	{"GET", "/api/v1/admin/audit/queue", "super.onboarding"},
	{"GET", "/api/v1/admin/audit/queue/:id", "super.onboarding"},
	{"GET", "/api/v1/admin/audit/reports", "platform.moderation.read"},
	{"GET", "/api/v1/admin/audit/sensitive-words", "platform.moderation.read"},
	{"GET", "/api/v1/admin/audit/statistics", "super.onboarding"},
	{"GET", "/api/v1/admin/auth/accounts", "super.security"},
	{"GET", "/api/v1/admin/auth/permissions", "super.security"},
	{"GET", "/api/v1/admin/auth/roles", "super.security"},
	{"GET", "/api/v1/admin/bookings", ""},
	{"GET", "/api/v1/admin/bookings/:id", ""},
	{"GET", "/api/v1/admin/bookings/:id/review", ""},
	{"GET", "/api/v1/admin/bookings/:id/status-log", ""},
	{"GET", "/api/v1/admin/bookings/report", ""},
	{"GET", "/api/v1/admin/diy/blessing-services", "platform.diy.read"},
	{"GET", "/api/v1/admin/diy/blessing-services/:id", "platform.diy.read"},
	{"GET", "/api/v1/admin/diy/designs", "platform.diy.read"},
	{"GET", "/api/v1/admin/diy/materials", "platform.diy.read"},
	{"GET", "/api/v1/admin/diy/materials/:id", "platform.diy.read"},
	{"GET", "/api/v1/admin/diy/orders", "platform.diy.read"},
	{"GET", "/api/v1/admin/diy/orders/:id", "platform.diy.read"},
	{"GET", "/api/v1/admin/files/backups", ""},
	{"GET", "/api/v1/admin/files/backups/:filename/download", ""},
	{"GET", "/api/v1/admin/finance/commission-config", "platform.finance.read"},
	{"GET", "/api/v1/admin/finance/overview", "platform.finance.read"},
	{"GET", "/api/v1/admin/finance/reports", "platform.finance.read"},
	{"GET", "/api/v1/admin/finance/settlements", "platform.finance.read"},
	{"GET", "/api/v1/admin/finance/settlements/:id", "platform.finance.read"},
	{"GET", "/api/v1/admin/finance/shop/reports", "platform.finance.read"},
	{"GET", "/api/v1/admin/finance/withdrawals", "platform.finance.read"},
	{"GET", "/api/v1/admin/logistics/express", "platform.logistics.read"},
	{"GET", "/api/v1/admin/logistics/freight-templates", "platform.logistics.read"},
	{"GET", "/api/v1/admin/logistics/tracks/:trackingNo", "platform.logistics.read"},
	{"GET", "/api/v1/admin/marketing/activities", "platform.marketing.read"},
	{"GET", "/api/v1/admin/marketing/banners", "platform.marketing.read"},
	{"GET", "/api/v1/admin/marketing/coupons", "platform.marketing.read"},
	{"GET", "/api/v1/admin/marketing/recommends", "platform.marketing.read"},
	{"GET", "/api/v1/admin/marketing/rewards/campaigns", "platform.marketing.read"},
	{"GET", "/api/v1/admin/marketing/rewards/campaigns/:id", "platform.marketing.read"},
	{"GET", "/api/v1/admin/marketing/rewards/campaigns/:id/audit", "platform.marketing.read"},
	{"GET", "/api/v1/admin/marketing/rewards/orders", "platform.marketing.read"},
	{"GET", "/api/v1/admin/masters/blessing-tasks", ""},
	{"GET", "/api/v1/admin/masters/blessing-tasks/:id", ""},
	{"GET", "/api/v1/admin/masters/bookings", ""},
	{"GET", "/api/v1/admin/masters/bookings/:id", ""},
	{"GET", "/api/v1/admin/masters/community/posts", ""},
	{"GET", "/api/v1/admin/masters/earnings/details", ""},
	{"GET", "/api/v1/admin/masters/earnings/summary", ""},
	{"GET", "/api/v1/admin/masters/profile", ""},
	{"GET", "/api/v1/admin/masters/reviews", ""},
	{"GET", "/api/v1/admin/masters/schedules", ""},
	{"GET", "/api/v1/admin/masters/service-tags", ""},
	{"GET", "/api/v1/admin/orders", "platform.orders.read"},
	{"GET", "/api/v1/admin/orders/:id", "platform.orders.read"},
	{"GET", "/api/v1/admin/orders/report", "platform.orders.read"},
	{"GET", "/api/v1/admin/orders/returns", "platform.orders.read"},
	{"GET", "/api/v1/admin/orders/returns/:id", "platform.orders.read"},
	{"GET", "/api/v1/admin/platform/beliefs", ""},
	{"GET", "/api/v1/admin/platform/beliefs/directory-tags", ""},
	{"GET", "/api/v1/admin/platform/beliefs/directory-tags/providers/:kind/:id", ""},
	{"GET", "/api/v1/admin/platform/community/comments", ""},
	{"GET", "/api/v1/admin/platform/community/posts", ""},
	{"GET", "/api/v1/admin/platform/intentions", ""},
	{"GET", "/api/v1/admin/platform/intentions/bindings", ""},
	{"GET", "/api/v1/admin/platform/masters", "platform.masters.read"},
	{"GET", "/api/v1/admin/platform/masters/:id", "platform.masters.read"},
	{"GET", "/api/v1/admin/platform/masters/:id/service-tags", "platform.masters.read"},
	{"GET", "/api/v1/admin/platform/masters/audits", "super.onboarding"},
	{"GET", "/api/v1/admin/platform/regions", ""},
	{"GET", "/api/v1/admin/platform/reviews/reports", ""},
	{"GET", "/api/v1/admin/platform/temples", "platform.temples.read"},
	{"GET", "/api/v1/admin/platform/temples/:id", "platform.temples.read"},
	{"GET", "/api/v1/admin/platform/temples/audits", "super.onboarding"},
	{"GET", "/api/v1/admin/products", "platform.products.read"},
	{"GET", "/api/v1/admin/products/:id", "platform.products.read"},
	{"GET", "/api/v1/admin/products/categories", "platform.products.read"},
	{"GET", "/api/v1/admin/reviews", ""},
	{"GET", "/api/v1/admin/reviews/:id", ""},
	{"GET", "/api/v1/admin/temples/blessing-tasks", ""},
	{"GET", "/api/v1/admin/temples/blessing-tasks/:id", ""},
	{"GET", "/api/v1/admin/temples/images", ""},
	{"GET", "/api/v1/admin/temples/info", ""},
	{"GET", "/api/v1/admin/temples/masters", ""},
	{"GET", "/api/v1/admin/temples/masters/:id", ""},
	{"GET", "/api/v1/admin/temples/reports", ""},
	{"GET", "/api/v1/admin/temples/services", ""},
	{"GET", "/api/v1/admin/temples/services/:id", ""},
	{"GET", "/api/v1/admin/users", "platform.users.read"},
	{"GET", "/api/v1/admin/users/:id", "platform.users.read"},
	{"POST", "/api/v1/admin/audit/sensitive-words", "platform.moderation.write"},
	{"POST", "/api/v1/admin/auth/accounts", "super.security"},
	{"POST", "/api/v1/admin/auth/roles", "super.security"},
	{"POST", "/api/v1/admin/diy/blessing-services", "platform.diy.write"},
	{"POST", "/api/v1/admin/diy/materials", "platform.diy.write"},
	{"POST", "/api/v1/admin/files/backups", ""},
	{"POST", "/api/v1/admin/files/backups/:filename/restore", ""},
	{"POST", "/api/v1/admin/finance/settlements/confirm/:id", "platform.finance.write"},
	{"POST", "/api/v1/admin/finance/withdrawals/apply", ""},
	{"POST", "/api/v1/admin/logistics/express", "platform.logistics.write"},
	{"POST", "/api/v1/admin/logistics/freight-templates", "platform.logistics.write"},
	{"POST", "/api/v1/admin/logistics/tracks/batch-sync", "platform.logistics.write"},
	{"POST", "/api/v1/admin/marketing/activities", "platform.marketing.write"},
	{"POST", "/api/v1/admin/marketing/banners", "platform.marketing.write"},
	{"POST", "/api/v1/admin/marketing/coupons", "platform.marketing.write"},
	{"POST", "/api/v1/admin/marketing/rewards/campaigns", "platform.marketing.write"},
	{"POST", "/api/v1/admin/marketing/rewards/campaigns/:id/cancel", "platform.marketing.write"},
	{"POST", "/api/v1/admin/marketing/rewards/campaigns/:id/draw", "platform.marketing.write"},
	{"POST", "/api/v1/admin/marketing/rewards/campaigns/:id/publish", "platform.marketing.write"},
	{"POST", "/api/v1/admin/marketing/rewards/campaigns/:id/stock", "platform.marketing.write"},
	{"POST", "/api/v1/admin/marketing/rewards/orders/:id/ship", "platform.marketing.write"},
	{"POST", "/api/v1/admin/masters/community/posts", ""},
	{"POST", "/api/v1/admin/platform/beliefs", ""},
	{"POST", "/api/v1/admin/platform/beliefs/directory-tags", ""},
	{"POST", "/api/v1/admin/platform/intentions", ""},
	{"POST", "/api/v1/admin/platform/masters", "super.onboarding"},
	{"POST", "/api/v1/admin/platform/regions", ""},
	{"POST", "/api/v1/admin/products", "platform.products.write"},
	{"POST", "/api/v1/admin/products/:id/skus", "platform.products.write"},
	{"POST", "/api/v1/admin/products/categories", "platform.products.write"},
	{"POST", "/api/v1/admin/reviews/:id/reply", ""},
	{"POST", "/api/v1/admin/reviews/:id/report", ""},
	{"POST", "/api/v1/admin/temples/apply", ""},
	{"POST", "/api/v1/admin/temples/images", ""},
	{"POST", "/api/v1/admin/temples/masters", ""},
	{"POST", "/api/v1/admin/temples/services", ""},
	{"POST", "/api/v1/auth/admin/login", ""},
	{"PUT", "/api/v1/admin/audit/queue/:id/approve", "super.onboarding"},
	{"PUT", "/api/v1/admin/audit/queue/:id/reject", "super.onboarding"},
	{"PUT", "/api/v1/admin/audit/reports/:id/handle", "platform.moderation.write"},
	{"PUT", "/api/v1/admin/auth/accounts/:id", "super.security"},
	{"PUT", "/api/v1/admin/auth/accounts/:id/status", "super.security"},
	{"PUT", "/api/v1/admin/auth/roles/:id", "super.security"},
	{"PUT", "/api/v1/admin/bookings/:id/cancel", ""},
	{"PUT", "/api/v1/admin/bookings/:id/complete", ""},
	{"PUT", "/api/v1/admin/bookings/:id/confirm", ""},
	{"PUT", "/api/v1/admin/bookings/:id/review/reply", ""},
	{"PUT", "/api/v1/admin/bookings/:id/timeout-cancel", ""},
	{"PUT", "/api/v1/admin/diy/blessing-services/:id", "platform.diy.write"},
	{"PUT", "/api/v1/admin/diy/designs/:id/status", "platform.diy.write"},
	{"PUT", "/api/v1/admin/diy/materials/:id", "platform.diy.write"},
	{"PUT", "/api/v1/admin/diy/materials/:id/status", "platform.diy.write"},
	{"PUT", "/api/v1/admin/diy/orders/:id/make-complete", "platform.diy.write"},
	{"PUT", "/api/v1/admin/diy/orders/:id/review", "platform.diy.write"},
	{"PUT", "/api/v1/admin/diy/orders/:id/ship", "platform.diy.write"},
	{"PUT", "/api/v1/admin/finance/commission-config/:id", "platform.finance.write"},
	{"PUT", "/api/v1/admin/finance/withdrawals/:id/audit", "platform.finance.write"},
	{"PUT", "/api/v1/admin/finance/withdrawals/:id/process", "platform.finance.write"},
	{"PUT", "/api/v1/admin/logistics/express/:id", "platform.logistics.write"},
	{"PUT", "/api/v1/admin/logistics/freight-templates/:id", "platform.logistics.write"},
	{"PUT", "/api/v1/admin/marketing/activities/:id", "platform.marketing.write"},
	{"PUT", "/api/v1/admin/marketing/banners/:id", "platform.marketing.write"},
	{"PUT", "/api/v1/admin/marketing/coupons/:id", "platform.marketing.write"},
	{"PUT", "/api/v1/admin/marketing/recommends/:id", "platform.marketing.write"},
	{"PUT", "/api/v1/admin/marketing/rewards/campaigns/:id", "platform.marketing.write"},
	{"PUT", "/api/v1/admin/masters/blessing-tasks/:id/accept", ""},
	{"PUT", "/api/v1/admin/masters/blessing-tasks/:id/complete", ""},
	{"PUT", "/api/v1/admin/masters/blessing-tasks/:id/reject", ""},
	{"PUT", "/api/v1/admin/masters/blessing-tasks/:id/start", ""},
	{"PUT", "/api/v1/admin/masters/bookings/:id/cancel", ""},
	{"PUT", "/api/v1/admin/masters/bookings/:id/complete", ""},
	{"PUT", "/api/v1/admin/masters/bookings/:id/confirm", ""},
	{"PUT", "/api/v1/admin/masters/bookings/:id/review/reply", ""},
	{"PUT", "/api/v1/admin/masters/bookings/:id/start", ""},
	{"PUT", "/api/v1/admin/masters/community/posts/:id", ""},
	{"PUT", "/api/v1/admin/masters/community/posts/:id/status", ""},
	{"PUT", "/api/v1/admin/masters/profile", ""},
	{"PUT", "/api/v1/admin/masters/schedules", ""},
	{"PUT", "/api/v1/admin/masters/service-tags", ""},
	{"PUT", "/api/v1/admin/orders/:id/ship", "platform.orders.write"},
	{"PUT", "/api/v1/admin/orders/returns/:id/refund", "super.refund"},
	{"PUT", "/api/v1/admin/orders/returns/:id/review", "platform.orders.write"},
	{"PUT", "/api/v1/admin/platform/beliefs/:code", ""},
	{"PUT", "/api/v1/admin/platform/beliefs/:code/status", ""},
	{"PUT", "/api/v1/admin/platform/beliefs/directory-tags/:code", ""},
	{"PUT", "/api/v1/admin/platform/beliefs/directory-tags/providers/:kind/:id", ""},
	{"PUT", "/api/v1/admin/platform/community/comments/:id/approve", ""},
	{"PUT", "/api/v1/admin/platform/community/comments/:id/reject", ""},
	{"PUT", "/api/v1/admin/platform/community/posts/:id/approve", ""},
	{"PUT", "/api/v1/admin/platform/community/posts/:id/reject", ""},
	{"PUT", "/api/v1/admin/platform/intentions/:code", ""},
	{"PUT", "/api/v1/admin/platform/intentions/:code/status", ""},
	{"PUT", "/api/v1/admin/platform/intentions/bindings", ""},
	{"PUT", "/api/v1/admin/platform/masters/:id", "super.onboarding"},
	{"PUT", "/api/v1/admin/platform/masters/:id/consultation", "platform.masters.write"},
	{"PUT", "/api/v1/admin/platform/masters/:id/service-tags", "platform.masters.write"},
	{"PUT", "/api/v1/admin/platform/masters/:id/status", "platform.masters.write"},
	{"PUT", "/api/v1/admin/platform/masters/audits/:id/pass", "super.onboarding"},
	{"PUT", "/api/v1/admin/platform/masters/audits/:id/reject", "super.onboarding"},
	{"PUT", "/api/v1/admin/platform/regions/:code", ""},
	{"PUT", "/api/v1/admin/platform/regions/providers/:kind/:id", ""},
	{"PUT", "/api/v1/admin/platform/reviews/reports/:id/handle", ""},
	{"PUT", "/api/v1/admin/platform/temples/:id/status", "platform.temples.write"},
	{"PUT", "/api/v1/admin/platform/temples/audits/:id/final-pass", "super.onboarding"},
	{"PUT", "/api/v1/admin/platform/temples/audits/:id/first-pass", "super.onboarding"},
	{"PUT", "/api/v1/admin/platform/temples/audits/:id/reject", "super.onboarding"},
	{"PUT", "/api/v1/admin/products/:id", "platform.products.write"},
	{"PUT", "/api/v1/admin/products/:id/skus/:skuId", "platform.products.write"},
	{"PUT", "/api/v1/admin/products/:id/status", "platform.products.write"},
	{"PUT", "/api/v1/admin/products/categories/:id", "platform.products.write"},
	{"PUT", "/api/v1/admin/temples/blessing-tasks/:id/assign", ""},
	{"PUT", "/api/v1/admin/temples/info", ""},
	{"PUT", "/api/v1/admin/temples/masters/:id", ""},
	{"PUT", "/api/v1/admin/temples/masters/:id/status", ""},
	{"PUT", "/api/v1/admin/temples/services/:id", ""},
	{"PUT", "/api/v1/admin/temples/services/:id/status", ""},
	{"PUT", "/api/v1/admin/users/:id/status", "platform.users.write"},
}

func Builtin(role string) bool {
	switch role {
	case "platform_super", "platform_service", "temple_admin", "master", "shop_admin", "customer", "master_applicant", "temple_applicant":
		return true
	}
	return false
}
func Scope(role string) string {
	switch role {
	case "platform_super", "platform_service":
		return "platform"
	case "temple_admin", "temple_applicant":
		return "temple"
	case "master", "master_applicant":
		return "master"
	case "shop_admin":
		return "shop"
	}
	return ""
}
func Assignable(code string) bool {
	for _, c := range Catalog {
		if c.Code == code {
			return c.Assignable
		}
	}
	return false
}
func Match(method, path string) (Route, bool) {
	// Static segments take precedence over parameter segments.
	var best Route
	score := -1
	for _, r := range Routes {
		if r.Method != method {
			continue
		}
		a, b := strings.Split(r.Template, "/"), strings.Split(path, "/")
		if len(a) != len(b) {
			continue
		}
		n := 0
		ok := true
		for i := range a {
			if strings.HasPrefix(a[i], ":") {
				if b[i] == "" {
					ok = false
					break
				}
			} else if a[i] != b[i] {
				ok = false
				break
			} else {
				n++
			}
		}
		if ok && n > score {
			best = r
			score = n
		}
	}
	return best, score >= 0
}
func Allowed(method, path string, permissions []string) bool {
	r, ok := Match(method, path)
	if !ok || !Assignable(r.Permission) {
		return false
	}
	for _, p := range permissions {
		if p == r.Permission {
			return true
		}
	}
	return false
}

package adminpolicy

import "testing"

func TestCatalogTranslationAccess(t *testing.T) {
	for _, tc := range []struct {
		method, kind, identity, client, scope, role string
		permissions                                 []string
		allow                                       bool
	}{
		{"GET", "reward-prize", "admin", "platform-admin", "platform", "marketing_delegate", []string{"platform.marketing.write"}, true},
		{"PUT", "reward-prize", "admin", "platform-admin", "platform", "marketing_delegate", []string{"platform.marketing.read"}, false},
		{"POST", "temple-service", "admin", "temple-admin", "temple", "temple_admin", nil, true},
		{"PUT", "temple-service", "admin", "platform-admin", "temple", "temple_admin", nil, false},
		{"POST", "points-product", "admin", "shop-admin", "shop", "shop_admin", nil, true},
		{"POST", "reward-campaign", "admin", "shop-admin", "shop", "shop_admin", nil, false},
		{"PUT", "belief", "service", "platform-admin", "platform", "platform_super", nil, false},
		{"PUT", "belief", "admin", "platform-admin", "platform", "platform_super", nil, true},
	} {
		path := CatalogTranslationPrefix + tc.kind + "/1"
		if got := CatalogTranslationAccess(tc.method, path, tc.identity, tc.client, tc.scope, []string{tc.role}, tc.permissions); got != tc.allow {
			t.Fatalf("%s %s with %s allowed=%t, want %t", tc.method, path, tc.role, got, tc.allow)
		}
	}
}

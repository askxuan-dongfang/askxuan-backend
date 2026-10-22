package adminpolicy

import "testing"

func TestFiniteCapabilitySurface(t *testing.T) {
	for _, r := range Routes {
		if !Assignable(r.Permission) {
			continue
		}
		if Allowed(r.Method, r.Template, nil) {
			t.Fatalf("empty role opened %s", r.Template)
		}
		if !Allowed(r.Method, r.Template, []string{r.Permission}) {
			t.Fatalf("grant not usable %s %s", r.Method, r.Template)
		}
		if Allowed("PATCH", r.Template, []string{r.Permission}) {
			t.Fatalf("unregistered method opened %s", r.Template)
		}
	}
	all := []string{}
	for _, c := range Catalog {
		if c.Assignable {
			all = append(all, c.Code)
		}
	}
	for _, path := range []string{"/api/v1/admin/temples/info", "/api/v1/admin/masters/profile", "/api/v1/admin/auth/roles", "/api/v1/admin/platform/beliefs", "/api/v1/admin/platform/regions", "/api/v1/admin/platform/service-orders/bookings", "/api/v1/admin/diy/creator-withdrawals/1/action", "/api/v1/admin/finance/payout-sandbox", "/api/v1/admin/products-private", "/api/v1/admin/products/1/new-action", "/api/v1/admin/orders/returns/1/refund", "/api/v1/admin/platform/masters/audits/1/pass", "/api/v1/admin/audit/queue", "/api/v1/admin/audit/statistics"} {
		for _, method := range []string{"GET", "POST", "PUT", "DELETE"} {
			if Allowed(method, path, all) {
				t.Fatalf("sensitive/unknown route delegated: %s %s", method, path)
			}
		}
	}
}
func TestStaticPathWinsOverParameter(t *testing.T) {
	r, ok := Match("GET", "/api/v1/admin/products/categories")
	if !ok || r.Template != "/api/v1/admin/products/categories" {
		t.Fatalf("wrong template %+v", r)
	}
}

package adminpolicy

import "strings"

func IsTempleStaff(role string) bool {
	return role == "temple_reception" || role == "temple_operations" || role == "temple_finance"
}
func IsTempleRole(role string) bool { return role == "temple_admin" || IsTempleStaff(role) }
func TemplePermissions(role string) []string {
	switch role {
	case "temple_reception":
		return []string{"temple.profile.read", "temple.people.read", "temple.services.read", "temple.schedules.read", "temple.bookings.read", "temple.bookings.write", "temple.exceptions.read", "temple.exceptions.write", "temple.reviews.read"}
	case "temple_operations":
		return []string{"temple.profile.read", "temple.profile.write", "temple.people.read", "temple.people.write", "temple.services.read", "temple.services.write", "temple.schedules.read", "temple.schedules.write", "temple.bookings.read", "temple.bookings.write", "temple.blessings.read", "temple.blessings.write", "temple.exceptions.read", "temple.exceptions.write", "temple.reviews.read", "temple.reviews.write", "temple.growth.read", "temple.growth.write"}
	case "temple_finance":
		return []string{"temple.finance.read"}
	}
	return []string{}
}

// Finite method/route grants. Never delegate owner, platform, customer, or master identity APIs.
var TempleRoutes = []Route{
	{"POST", "/api/v1/files/upload", "temple.profile.write"},
	{"GET", "/api/v1/auth/access", "session"}, {"POST", "/api/v1/auth/logout", "session"},
	{"GET", "/api/v1/service-types", "temple.services.read"}, {"GET", "/api/v1/beliefs", "temple.profile.read"}, {"GET", "/api/v1/intentions/tags", "temple.services.read"},
	{"GET", "/api/v1/admin/temples/info", "temple.profile.read"}, {"PUT", "/api/v1/admin/temples/info", "temple.profile.write"},
	{"GET", "/api/v1/admin/temples/images", "temple.profile.read"}, {"POST", "/api/v1/admin/temples/images", "temple.profile.write"}, {"DELETE", "/api/v1/admin/temples/images/:id", "temple.profile.write"},
	{"GET", "/api/v1/admin/temples/masters", "temple.people.read"}, {"GET", "/api/v1/admin/temples/masters/:id", "temple.people.read"},
	{"POST", "/api/v1/admin/temples/masters", "temple.people.write"}, {"PUT", "/api/v1/admin/temples/masters/:id", "temple.people.write"}, {"PUT", "/api/v1/admin/temples/masters/:id/status", "temple.people.write"},
	{"GET", "/api/v1/admin/temples/schedules", "temple.schedules.read"}, {"PUT", "/api/v1/admin/temples/schedules", "temple.schedules.write"},
	{"GET", "/api/v1/admin/temples/services", "temple.services.read"}, {"GET", "/api/v1/admin/temples/services/:id", "temple.services.read"},
	{"POST", "/api/v1/admin/temples/services", "temple.services.write"}, {"PUT", "/api/v1/admin/temples/services/:id", "temple.services.write"}, {"PUT", "/api/v1/admin/temples/services/:id/status", "temple.services.write"},
	{"GET", "/api/v1/admin/temples/blessing-tasks", "temple.blessings.read"}, {"GET", "/api/v1/admin/temples/blessing-tasks/:id", "temple.blessings.read"}, {"PUT", "/api/v1/admin/temples/blessing-tasks/:id/assign", "temple.blessings.write"},
	{"GET", "/api/v1/admin/bookings/report", "temple.finance.read"},
	{"GET", "/api/v1/admin/bookings", "temple.bookings.read"}, {"GET", "/api/v1/admin/bookings/:id", "temple.bookings.read"}, {"GET", "/api/v1/admin/bookings/:id/status-log", "temple.bookings.read"}, {"GET", "/api/v1/admin/bookings/:id/review", "temple.bookings.read"},
	{"PUT", "/api/v1/admin/bookings/:id/confirm", "temple.bookings.write"}, {"PUT", "/api/v1/admin/bookings/:id/cancel", "temple.bookings.write"}, {"PUT", "/api/v1/admin/bookings/:id/start", "temple.bookings.write"}, {"PUT", "/api/v1/admin/bookings/:id/timeout-cancel", "temple.bookings.write"}, {"PUT", "/api/v1/admin/bookings/:id/review/reply", "temple.reviews.write"},
	{"GET", "/api/v1/bookings/journeys", "temple.bookings.read"}, {"GET", "/api/v1/bookings/:id/fulfillment", "temple.bookings.read"}, {"GET", "/api/v1/bookings/:id/receipt-files/:file", "temple.bookings.read"},
	{"POST", "/api/v1/bookings/:id/progress/update", "temple.bookings.write"}, {"POST", "/api/v1/bookings/:id/receipts", "temple.bookings.write"}, {"POST", "/api/v1/bookings/:id/receipt-files", "temple.bookings.write"},
	{"GET", "/api/v1/chats", "temple.bookings.read"}, {"GET", "/api/v1/chats/unread", "temple.bookings.read"}, {"GET", "/api/v1/chats/:id", "temple.bookings.read"}, {"GET", "/api/v1/chats/:id/messages", "temple.bookings.read"}, {"GET", "/api/v1/chats/:id/attachments/:attachment", "temple.bookings.read"},
	{"POST", "/api/v1/chats/:id/read", "temple.bookings.write"}, {"POST", "/api/v1/chats/:id/messages", "temple.bookings.write"}, {"POST", "/api/v1/chats/:id/attachments", "temple.bookings.write"},
	{"GET", "/api/v1/admin/temples/exceptions", "temple.exceptions.read"}, {"GET", "/api/v1/admin/temples/exceptions/:id", "temple.exceptions.read"}, {"POST", "/api/v1/admin/temples/exceptions/:id/notes", "temple.exceptions.write"},
	{"GET", "/api/v1/admin/reviews", "temple.reviews.read"}, {"GET", "/api/v1/admin/reviews/:id", "temple.reviews.read"}, {"POST", "/api/v1/admin/reviews/:id/reply", "temple.reviews.write"}, {"POST", "/api/v1/admin/reviews/:id/report", "temple.reviews.write"},
	{"GET", "/api/v1/finance/wallet/temple", "temple.finance.read"}, {"GET", "/api/v1/finance/wallet/temple/report", "temple.finance.read"},
	{"GET", "/api/v1/payments/growth/provider/rules", "temple.growth.read"}, {"GET", "/api/v1/payments/growth/provider/policies", "temple.growth.read"}, {"GET", "/api/v1/payments/growth/provider/bills", "temple.growth.read"}, {"POST", "/api/v1/payments/growth/provider/consent", "temple.growth.write"},
}

func TempleAllowed(method, path, role string, permissions []string) bool {
	if !IsTempleStaff(role) {
		return false
	}
	// Static route segments must win over :id (notably the financial report).
	var best Route
	score := -1
	for _, r := range TempleRoutes {
		if r.Method != method {
			continue
		}
		a, b := strings.Split(r.Template, "/"), strings.Split(path, "/")
		if len(a) != len(b) {
			continue
		}
		n, ok := 0, true
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
	if score < 0 {
		return false
	}
	if best.Permission == "session" {
		return true
	}
	// The signed grants and immutable job template must both authorize the action.
	for _, fixed := range TemplePermissions(role) {
		if fixed == best.Permission {
			for _, p := range permissions {
				if p == fixed {
					return true
				}
			}
		}
	}
	return false
}

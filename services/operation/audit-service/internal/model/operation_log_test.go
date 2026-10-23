package model

import (
	"context"
	"strings"
	"testing"
	"time"
)

func testOperationStart() OperationStart {
	return OperationStart{RequestID: "0123456789abcdef0123456789abcdef", ActorID: 7, ActorRoles: []string{"platform_super"}, Action: "platform.products.write", Method: "PUT", RouteTemplate: "/api/v1/admin/products/:id", TargetType: "products", TargetID: "17"}
}

func TestOperationStartRejectsUnsafeMetadataBeforeStorage(t *testing.T) {
	store := NewOperationStore(nil)
	valid := testOperationStart()
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	cases := map[string]func(*OperationStart){
		"request id":          func(r *OperationStart) { r.RequestID = "not-a-request" },
		"zero actor":          func(r *OperationStart) { r.ActorID = 0 },
		"missing roles":       func(r *OperationStart) { r.ActorRoles = nil },
		"duplicate roles":     func(r *OperationStart) { r.ActorRoles = []string{"platform_super", "platform_super"} },
		"unsafe role":         func(r *OperationStart) { r.ActorRoles = []string{"admin@example.test"} },
		"too many roles":      func(r *OperationStart) { r.ActorRoles = make([]string, 33) },
		"unsafe action":       func(r *OperationStart) { r.Action = "update password=secret" },
		"uncatalogued action": func(r *OperationStart) { r.Action = "arbitrary.private.value" },
		"oversized action":    func(r *OperationStart) { r.Action = strings.Repeat("x", 65) },
		"raw path":            func(r *OperationStart) { r.RouteTemplate = "/api/v1/admin/products/17" },
		"raw query":           func(r *OperationStart) { r.RouteTemplate += "?email=private" },
		"unknown route":       func(r *OperationStart) { r.RouteTemplate = "/private/raw" },
		"read method":         func(r *OperationStart) { r.Method = "GET" },
		"unsafe target type":  func(r *OperationStart) { r.TargetType = "contact info" },
		"email target":        func(r *OperationStart) { r.TargetID = "admin@example.test" },
		"oversized target":    func(r *OperationStart) { r.TargetID = strings.Repeat("a", 129) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			req := valid
			mutate(&req)
			// A nil connection proves invalid metadata cannot reach a database call.
			if _, err := store.Start(context.Background(), req); err == nil {
				t.Fatal("accepted unsafe metadata")
			}
		})
	}
	for _, route := range []string{"/api/v1/admin/*", "/api/v1/ai/admin/*", "/api/v1/payments/growth/admin/*", "/api/v1/auth/onboarding/*"} {
		valid.RouteTemplate = route
		if err := valid.Validate(); err != nil {
			t.Fatalf("fixed generic template rejected: %s", route)
		}
	}
}

func TestOperationFinishRejectsUnsafeStateBeforeStorage(t *testing.T) {
	store := NewOperationStore(nil)
	valid := OperationFinish{RequestID: testOperationStart().RequestID, Result: "success", HTTPStatus: 200, DurationMs: 9}
	for _, result := range []string{"success", "denied", "failed", "unknown"} {
		valid.Result = result
		if err := valid.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	cases := []OperationFinish{valid, valid, valid, valid, valid, valid, valid}
	cases[0].Result = "pending"
	cases[1].RequestID = "invalid"
	cases[2].HTTPStatus = 0
	cases[3].HTTPStatus = 600
	cases[4].BusinessCode = -1
	cases[4].Result = "success"
	cases[5].DurationMs = -1
	cases[6].DurationMs = 86400001
	for _, req := range cases {
		if _, err := store.Finish(context.Background(), req); err == nil {
			t.Fatal("accepted invalid finalization")
		}
	}
}

func TestHistoricalTempleTeamAuditRecordRemainsReadable(t *testing.T) {
	r := testOperationStart()
	r.ActorRoles = []string{"temple_admin"}
	r.Action, r.RouteTemplate, r.TargetType = "temple.team.manage", "/api/v1/auth/temple/team", "admin_account"
	for _, method := range []string{"POST", "PUT"} {
		r.Method = method
		if err := r.Validate(); err != nil {
			t.Fatal(method, err)
		}
	}
	r.Method = "DELETE"
	if r.Validate() == nil {
		t.Fatal("unimplemented team deletion accepted")
	}
	r.Method, r.RouteTemplate = "POST", "/api/v1/auth/temple/team?password=secret"
	if r.Validate() == nil {
		t.Fatal("raw team query accepted into audit")
	}
}

func TestOperationFilterValidationAndBoundParameters(t *testing.T) {
	filter := OperationFilter{ActorID: 7, Action: "platform.products.write", Method: "PUT", TargetType: "products", Result: "pending", Page: 2, Size: 20, StartTime: time.Date(2026, 10, 22, 8, 0, 0, 0, time.FixedZone("test", 8*3600))}
	if err := filter.Validate(); err != nil {
		t.Fatal(err)
	}
	where, args := operationWhere(filter)
	if len(args) != 6 || strings.Contains(where, filter.Action) || strings.Contains(where, filter.Result) || args[5] != "2026-10-22 00:00:00" {
		t.Fatalf("filters must use bound UTC parameters: %s %#v", where, args)
	}
	for _, mutate := range []func(*OperationFilter){
		func(f *OperationFilter) { f.Page = 0 }, func(f *OperationFilter) { f.Size = 101 },
		func(f *OperationFilter) { f.Action = "' OR 1=1" }, func(f *OperationFilter) { f.Result = "complete" },
		func(f *OperationFilter) { f.EndTime = f.StartTime.Add(-time.Minute) },
	} {
		bad := filter
		mutate(&bad)
		if _, err := NewOperationStore(nil).List(context.Background(), bad); err == nil {
			t.Fatal("accepted invalid filter")
		}
	}
}

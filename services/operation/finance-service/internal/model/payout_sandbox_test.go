package model

import "testing"

func TestPayoutSandboxStateMachineRejectsUnknownRetryAndTerminalWrites(t *testing.T) {
	for _, tc := range []struct {
		from, action, outcome, to string
		versions                  int64
		attempts                  int
	}{
		{SandboxPendingReview, "approve", "", SandboxApproved, 1, 0}, {SandboxPendingReview, "reject", "", SandboxRejected, 1, 0},
		{SandboxApproved, "submit", SandboxSuccess, SandboxSuccess, 2, 1}, {SandboxApproved, "submit", SandboxFailed, SandboxFailed, 2, 1},
		{SandboxApproved, "submit", SandboxUnknown, SandboxUnknown, 2, 1}, {SandboxFailed, "retry", SandboxUnknown, SandboxUnknown, 2, 1},
		{SandboxUnknown, "resolve", SandboxSuccess, SandboxSuccess, 1, 0}, {SandboxUnknown, "resolve", SandboxFailed, SandboxFailed, 1, 0},
	} {
		to, v, a, err := sandboxNext(tc.from, tc.action, tc.outcome)
		if err != nil || to != tc.to || v != tc.versions || a != tc.attempts {
			t.Fatalf("transition %+v: %s %d %d %v", tc, to, v, a, err)
		}
	}
	for _, status := range []string{SandboxPendingReview, SandboxApproved, SandboxRejected, SandboxSuccess, SandboxFailed, SandboxUnknown} {
		for _, action := range []string{"submit", "retry"} {
			allowed := status == SandboxApproved && action == "submit" || status == SandboxFailed && action == "retry"
			_, _, _, err := sandboxNext(status, action, SandboxSuccess)
			if allowed != (err == nil) {
				t.Fatalf("%s %s allowed=%v err=%v", status, action, allowed, err)
			}
		}
	}
	if _, _, _, err := sandboxNext(SandboxUnknown, "resolve", SandboxUnknown); err == nil {
		t.Fatal("unknown query result confirmed without a definitive outcome")
	}
}

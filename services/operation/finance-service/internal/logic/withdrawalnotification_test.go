package logic

import (
	"testing"

	"github.com/askxuan/finance-service/internal/model"
)

func TestWithdrawalNotificationRecipientSeparatesMasterIdentity(t *testing.T) {
	if got := withdrawalNotificationRecipient(model.SettleTypeMaster, "1"); got != "m_1" {
		t.Fatalf("master recipient = %q, want m_1", got)
	}
	if got := withdrawalNotificationRecipient("temple", "1"); got != "1" {
		t.Fatalf("non-master recipient = %q, want 1", got)
	}
}

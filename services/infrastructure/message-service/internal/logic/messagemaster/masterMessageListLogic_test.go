package messagemaster

import "testing"

func TestMasterRecipientIDUsesSeparateNamespace(t *testing.T) {
	if got := masterRecipientID(1); got != "m_1" {
		t.Fatalf("masterRecipientID(1) = %q, want m_1", got)
	}
}

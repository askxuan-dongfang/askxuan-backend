package model

import (
	"context"
	"testing"
)

func TestExperienceReceiptsNeverTouchLedger(t *testing.T) {
	// No database configured: both paths must return before any ledger write.
	r := PaymentReceipt{PaymentNo: "PAY-case", SourceType: "shop_order", SourceNo: "EXO-case", Amount: 1000}
	if err := RecordPlatformReceipt(context.Background(), r); err != nil {
		t.Fatal(err)
	}
	if err := RecordPlatformRefund(context.Background(), r); err != nil {
		t.Fatal(err)
	}
}

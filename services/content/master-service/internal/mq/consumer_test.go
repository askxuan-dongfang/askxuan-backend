package mq

import "testing"

func TestParseSettlementAccrued(t *testing.T) {
	evt, ok := ParseSettlementAccrued([]byte(`{"eventType":"settlement.accrued","sourceType":"booking","sourceNo":"B1","targetType":"master","targetId":"M1","amount":176}`))
	if !ok || evt.SourceNo != "B1" || evt.TargetId != "M1" || evt.Amount != 176 {
		t.Fatalf("unexpected event: ok=%t evt=%+v", ok, evt)
	}
}

func TestParseSettlementAccruedRejectsDirectBookingCompletion(t *testing.T) {
	_, ok := ParseSettlementAccrued([]byte(`{"bookingId":"B1","masterId":"M1","totalFee":200,"action":"completed"}`))
	if ok {
		t.Fatal("booking completion must not directly credit master earnings")
	}
}

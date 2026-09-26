package main

import (
	"github.com/askxuan/message-service/internal/svc"
	"testing"
)

func TestReviewRepairDoesNotNotify(t *testing.T) {
	if err := handleBookingNotify(&svc.ServiceContext{})([]byte(`{"bookingId":"BTEST","userId":"1","action":"review_synced"}`)); err != nil {
		t.Fatal(err)
	}
}
func TestReviewReplyMessage(t *testing.T) {
	title, _ := buildBookingMessage("review_replied", "BTEST")
	if title != "服务方回复了评价" {
		t.Fatal(title)
	}
}

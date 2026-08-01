package logic

import (
	"testing"

	"github.com/askxuan/booking-service/internal/model"
)

func TestParseOpenIMBookingPair(t *testing.T) {
	tests := []struct {
		name        string
		sendID      string
		recvID      string
		userID      string
		masterID    int64
		bookingPair bool
		wantErr     bool
	}{
		{name: "customer to master", sendID: "u_42", recvID: "m_7", userID: "42", masterID: 7, bookingPair: true},
		{name: "master to customer", sendID: "m_7", recvID: "u_42", userID: "42", masterID: 7, bookingPair: true},
		{name: "system message bypasses booking gate", sendID: "system", recvID: "u_42", bookingPair: false},
		{name: "master code is not an OpenIM master id", sendID: "u_42", recvID: "m_M001", bookingPair: true, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID, masterID, bookingPair, err := parseOpenIMBookingPair(tt.sendID, tt.recvID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if userID != tt.userID || masterID != tt.masterID || bookingPair != tt.bookingPair {
				t.Fatalf("got (%q,%d,%v), want (%q,%d,%v)", userID, masterID, bookingPair, tt.userID, tt.masterID, tt.bookingPair)
			}
		})
	}
}

func TestBookingMessageMarker(t *testing.T) {
	bookingID, clientID, ok := parseBookingMessageMarker("askxuan-booking:B202607310001:ios-uuid-1")
	if !ok || bookingID != "B202607310001" || clientID != "ios-uuid-1" {
		t.Fatalf("unexpected marker result: %q %q %v", bookingID, clientID, ok)
	}
	if _, _, ok := parseBookingMessageMarker("untrusted"); ok {
		t.Fatal("untrusted ex must not be accepted as a booking marker")
	}
}

func TestDecodeOpenIMText(t *testing.T) {
	if got := decodeOpenIMText(`{"content":"付款后可沟通"}`); got != "付款后可沟通" {
		t.Fatalf("decoded text = %q", got)
	}
	if got := decodeOpenIMText("plain text"); got != "plain text" {
		t.Fatalf("plain text = %q", got)
	}
}

func TestBookingHasChatEntitlement(t *testing.T) {
	paid := &model.Booking{UserId: "42", MasterId: "M001", PaymentStatus: model.PaymentStatusSuccess, Status: model.StatusPending}
	if !bookingHasChatEntitlement(paid, "42") {
		t.Fatal("active paid booking must unlock chat")
	}
	for name, booking := range map[string]*model.Booking{
		"wrong owner": {UserId: "7", MasterId: "M001", PaymentStatus: model.PaymentStatusSuccess, Status: model.StatusPending},
		"unpaid":      {UserId: "42", MasterId: "M001", PaymentStatus: model.PaymentStatusPending, Status: model.StatusPendingPayment},
		"cancelled":   {UserId: "42", MasterId: "M001", PaymentStatus: model.PaymentStatusSuccess, Status: model.StatusCancelled},
		"no master":   {UserId: "42", PaymentStatus: model.PaymentStatusSuccess, Status: model.StatusPending},
	} {
		t.Run(name, func(t *testing.T) {
			if bookingHasChatEntitlement(booking, "42") {
				t.Fatal("booking must not unlock chat")
			}
		})
	}
}

func TestChatDeliveryDecision(t *testing.T) {
	if deliver, content := chatDeliveryDecision(true, &model.BookingChatMessage{}, "new"); !deliver || content != "new" {
		t.Fatalf("new message decision = %v %q", deliver, content)
	}
	for _, status := range []string{"pending", "sent"} {
		if deliver, _ := chatDeliveryDecision(false, &model.BookingChatMessage{Status: status, Content: "original"}, "changed"); deliver {
			t.Fatalf("duplicate %s message must not be delivered twice", status)
		}
	}
	if deliver, content := chatDeliveryDecision(false, &model.BookingChatMessage{Status: "failed", Content: "original"}, "changed"); !deliver || content != "original" {
		t.Fatalf("failed retry decision = %v %q", deliver, content)
	}
}

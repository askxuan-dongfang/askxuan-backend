package messagecustomer

import (
	"context"
	"errors"
	"testing"

	"github.com/askxuan/common"
	"github.com/askxuan/message-service/internal/types"
)

func TestLegacyConsultSendIsClosed(t *testing.T) {
	logic := NewSendMessageLogic(context.Background(), nil)
	resp, err := logic.SendMessage(&types.SendMessageReq{
		ConversationId: "M001",
		UserId:         "1",
		Content:        "must not bypass paid booking chat",
	})
	if resp != nil {
		t.Fatalf("expected no response, got %#v", resp)
	}
	if !errors.Is(err, common.ErrBookingChatUnavailable) {
		t.Fatalf("expected ErrBookingChatUnavailable, got %v", err)
	}
}

package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/askxuan/order-service/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func TestOrderListRequestDoesNotRequireUserIDQuery(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/orders?page=1&size=50", nil)
	var parsed types.OrderListReq
	if err := httpx.Parse(req, &parsed); err != nil {
		t.Fatalf("parse JWT-scoped order list request: %v", err)
	}
	if parsed.UserId != "" || parsed.Page != 1 || parsed.Size != 50 {
		t.Fatalf("unexpected parsed request: %+v", parsed)
	}
}

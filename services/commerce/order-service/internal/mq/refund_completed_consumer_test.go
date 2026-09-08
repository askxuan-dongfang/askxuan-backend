package mq

import (
	"context"
	"errors"
	"testing"

	"github.com/askxuan/order-service/internal/model"
)

type refundOrderStub struct {
	model.ShopOrderModel
	failure error
}

func (s refundOrderStub) FindByOrderNo(context.Context, string) (*model.ShopOrder, error) {
	return &model.ShopOrder{Id: 901}, s.failure
}

type refundPageStub struct {
	model.ReturnOrderModel
	pages int
}

func (s *refundPageStub) FindList(_ context.Context, _ string, page, size int) ([]*model.ReturnOrder, int64, error) {
	s.pages++
	if page == 2 {
		return []*model.ReturnOrder{{Id: 99, OrderId: 901}}, 51, nil
	}
	rows := make([]*model.ReturnOrder, 50)
	for i := range rows {
		rows[i] = &model.ReturnOrder{Id: int64(i + 1), OrderId: int64(i + 1)}
	}
	return rows, 51, nil
}
func TestRefundCompletionFindsOlderReturnAndRetriesFailures(t *testing.T) {
	event := []byte(`{"orderNo":"ORDER-901","orderType":"shop_order","action":"refunded"}`)
	pages := &refundPageStub{}
	temporary := errors.New("inventory temporarily unavailable")
	h := NewRefundCompletedHandler(RefundCompletedDeps{ShopOrderModel: refundOrderStub{}, ReturnOrderModel: pages, Finalize: func(_ context.Context, o *model.ShopOrder, r *model.ReturnOrder) error {
		if o.Id != 901 || r.Id != 99 {
			t.Fatalf("incorrect order/return: %d/%d", o.Id, r.Id)
		}
		return temporary
	}})
	if err := h(event); !errors.Is(err, temporary) {
		t.Fatalf("finalization must retry: %v", err)
	}
	if pages.pages != 2 {
		t.Fatalf("older return omitted, pages=%d", pages.pages)
	}
	lookup := NewRefundCompletedHandler(RefundCompletedDeps{ShopOrderModel: refundOrderStub{failure: temporary}})
	if err := lookup(event); !errors.Is(err, temporary) {
		t.Fatalf("lookup failure must retry: %v", err)
	}
	scoped := NewRefundCompletedHandler(RefundCompletedDeps{ShopOrderModel: refundOrderStub{}, FindRefunding: func(_ context.Context, id int64) (*model.ReturnOrder, error) {
		if id != 901 {
			t.Fatalf("wrong scoped order %d", id)
		}
		return nil, nil
	}})
	if err := scoped(event); err != nil {
		t.Fatal(err)
	}
}

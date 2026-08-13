package server

import (
	"context"
	"errors"
	"testing"

	"github.com/askxuan/payment-service/internal/config"
	"github.com/askxuan/payment-service/internal/model"
	"github.com/askxuan/payment-service/internal/mq"
	"github.com/askxuan/payment-service/internal/svc"
	"github.com/askxuan/payment-service/rpc/payment"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type bookingPaymentModelStub struct {
	payment *model.Payment
}

func (m *bookingPaymentModelStub) Insert(context.Context, *model.Payment) (*model.Payment, error) {
	return nil, errors.New("unexpected Insert")
}
func (m *bookingPaymentModelStub) FindOne(context.Context, int64) (*model.Payment, error) {
	return nil, errors.New("unexpected FindOne")
}
func (m *bookingPaymentModelStub) FindByPaymentNo(context.Context, string) (*model.Payment, error) {
	return nil, errors.New("unexpected FindByPaymentNo")
}
func (m *bookingPaymentModelStub) FindByIdempotencyKey(context.Context, string) (*model.Payment, error) {
	return m.payment, nil
}
func (m *bookingPaymentModelStub) FindByOrder(context.Context, string, string) (*model.Payment, error) {
	return nil, errors.New("unexpected FindByOrder")
}
func (m *bookingPaymentModelStub) UpdateStatus(context.Context, int64, string, string) (*model.Payment, error) {
	return nil, errors.New("unexpected UpdateStatus")
}

type paymentPublisherStub struct {
	event mq.PaymentNotify
	err   error
}

func (p *paymentPublisherStub) Publish(_ context.Context, event mq.PaymentNotify) error {
	p.event = event
	return p.err
}

func TestAutoPayBookingRepublishesSuccessfulExistingPayment(t *testing.T) {
	paid := &model.Payment{PaymentNo: "PAY1", UserId: "1", OrderType: "booking", OrderNo: "B1", Amount: 201, Channel: "mock", Status: "success"}
	publisher := &paymentPublisherStub{}
	server := &PaymentBookingServer{
		svcCtx:    &svc.ServiceContext{Config: config.Config{Provider: "mock"}, PaymentModel: &bookingPaymentModelStub{payment: paid}},
		publisher: publisher,
	}

	got, err := server.AutoPayBooking(context.Background(), &payment.AutoPayBookingReq{OrderNo: "B1", UserId: "1", Amount: 201, IdempotencyKey: "booking:B1"})
	if err != nil {
		t.Fatal(err)
	}
	if got.PaymentNo != "PAY1" || publisher.event.OrderNo != "B1" || publisher.event.Action != "success" {
		t.Fatalf("payment=%+v event=%+v", got, publisher.event)
	}
}

func TestAutoPayBookingReturnsUnavailableWhenRepublishFails(t *testing.T) {
	paid := &model.Payment{PaymentNo: "PAY1", UserId: "1", OrderType: "booking", OrderNo: "B1", Amount: 201, Channel: "mock", Status: "success"}
	server := &PaymentBookingServer{
		svcCtx:    &svc.ServiceContext{Config: config.Config{Provider: "mock"}, PaymentModel: &bookingPaymentModelStub{payment: paid}},
		publisher: &paymentPublisherStub{err: errors.New("rabbit unavailable")},
	}

	_, err := server.AutoPayBooking(context.Background(), &payment.AutoPayBookingReq{OrderNo: "B1", UserId: "1", Amount: 201, IdempotencyKey: "booking:B1"})
	if status.Code(err) != codes.Unavailable {
		t.Fatalf("code=%s err=%v", status.Code(err), err)
	}
}

var _ model.PaymentModel = (*bookingPaymentModelStub)(nil)

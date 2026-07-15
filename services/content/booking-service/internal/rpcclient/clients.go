package rpcclient

import (
	"context"
	"time"

	masterrpc "github.com/askxuan/booking-service/rpc/master"
	paymentrpc "github.com/askxuan/booking-service/rpc/payment"
	templerpc "github.com/askxuan/booking-service/rpc/temple"
	"github.com/zeromicro/go-zero/zrpc"
)

type TempleClient interface {
	GetBookingService(ctx context.Context, templeCode, serviceCode string) (*templerpc.BookingService, error)
}
type MasterClient interface {
	GetByCode(ctx context.Context, code string) (*masterrpc.BookingMaster, error)
	GetByID(ctx context.Context, id int64) (*masterrpc.BookingMaster, error)
}
type PaymentClient interface {
	AutoPayBooking(ctx context.Context, orderNo, userId string, amount float64) (*paymentrpc.BookingPayment, error)
	GetOrderPayment(ctx context.Context, orderNo string) (*paymentrpc.BookingPayment, error)
}

type templeClient struct {
	client templerpc.TempleBookingServiceClient
}

func NewTempleClient(c zrpc.Client) TempleClient {
	return &templeClient{client: templerpc.NewTempleBookingServiceClient(c.Conn())}
}
func (c *templeClient) GetBookingService(ctx context.Context, templeCode, serviceCode string) (*templerpc.BookingService, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return c.client.GetBookingService(ctx, &templerpc.GetBookingServiceReq{TempleCode: templeCode, ServiceCode: serviceCode})
}

type masterClient struct {
	client masterrpc.MasterBookingServiceClient
}

func NewMasterClient(c zrpc.Client) MasterClient {
	return &masterClient{client: masterrpc.NewMasterBookingServiceClient(c.Conn())}
}
func (c *masterClient) GetByCode(ctx context.Context, code string) (*masterrpc.BookingMaster, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return c.client.GetBookingMaster(ctx, &masterrpc.GetBookingMasterReq{MasterCode: code})
}
func (c *masterClient) GetByID(ctx context.Context, id int64) (*masterrpc.BookingMaster, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return c.client.GetBookingMaster(ctx, &masterrpc.GetBookingMasterReq{MasterId: id})
}

type paymentClient struct {
	client paymentrpc.PaymentBookingServiceClient
}

func NewPaymentClient(c zrpc.Client) PaymentClient {
	return &paymentClient{client: paymentrpc.NewPaymentBookingServiceClient(c.Conn())}
}
func (c *paymentClient) AutoPayBooking(ctx context.Context, orderNo, userId string, amount float64) (*paymentrpc.BookingPayment, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return c.client.AutoPayBooking(ctx, &paymentrpc.AutoPayBookingReq{OrderNo: orderNo, UserId: userId, Amount: amount, IdempotencyKey: "booking:" + orderNo})
}
func (c *paymentClient) GetOrderPayment(ctx context.Context, orderNo string) (*paymentrpc.BookingPayment, error) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return c.client.GetOrderPayment(ctx, &paymentrpc.GetOrderPaymentReq{OrderType: "booking", OrderNo: orderNo})
}

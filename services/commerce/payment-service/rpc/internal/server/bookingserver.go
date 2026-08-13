package server

import (
	"context"
	"fmt"
	"time"

	"github.com/askxuan/payment-service/internal/model"
	"github.com/askxuan/payment-service/internal/mq"
	"github.com/askxuan/payment-service/internal/svc"
	"github.com/askxuan/payment-service/rpc/payment"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PaymentBookingServer struct {
	payment.UnimplementedPaymentBookingServiceServer
	svcCtx    *svc.ServiceContext
	publisher paymentEventPublisher
}

func NewPaymentBookingServer(svcCtx *svc.ServiceContext) *PaymentBookingServer {
	return &PaymentBookingServer{svcCtx: svcCtx, publisher: svcCtx.MqProducer}
}

type paymentEventPublisher interface {
	Publish(context.Context, mq.PaymentNotify) error
}

func toRpc(p *model.Payment) *payment.BookingPayment {
	return &payment.BookingPayment{Id: p.Id, PaymentNo: p.PaymentNo, OrderType: p.OrderType, OrderNo: p.OrderNo, Amount: p.Amount, Channel: p.Channel, Status: p.Status, TradeNo: p.TradeNo, Simulated: p.Channel == "mock"}
}

func (s *PaymentBookingServer) AutoPayBooking(ctx context.Context, req *payment.AutoPayBookingReq) (*payment.BookingPayment, error) {
	return s.autoPayOrder(ctx, model.OrderTypeBooking, req.OrderNo, req.UserId, req.Amount, req.IdempotencyKey)
}

func (s *PaymentBookingServer) AutoPayOrder(ctx context.Context, req *payment.AutoPayOrderReq) (*payment.BookingPayment, error) {
	if req.OrderType != model.OrderTypeBooking && req.OrderType != model.OrderTypeConsultation {
		return nil, status.Error(codes.InvalidArgument, "unsupported order type")
	}
	return s.autoPayOrder(ctx, req.OrderType, req.OrderNo, req.UserId, req.Amount, req.IdempotencyKey)
}

func (s *PaymentBookingServer) autoPayOrder(ctx context.Context, orderType, orderNo, userID string, amount float64, idempotencyKey string) (*payment.BookingPayment, error) {
	if s.svcCtx.Config.Provider != "mock" {
		return nil, status.Error(codes.FailedPrecondition, "mock payment disabled")
	}
	if orderNo == "" || userID == "" || amount <= 0 || idempotencyKey == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid payment request")
	}
	if existing, err := s.svcCtx.PaymentModel.FindByIdempotencyKey(ctx, idempotencyKey); err == nil {
		if existing.OrderType != orderType || existing.OrderNo != orderNo || existing.UserId != userID || existing.Amount != amount {
			return nil, status.Error(codes.AlreadyExists, "idempotency key payload mismatch")
		}
		if err := s.publishBookingSuccess(ctx, existing); err != nil {
			return nil, status.Error(codes.Unavailable, "payment notification unavailable")
		}
		return toRpc(existing), nil
	}
	paymentNo := "PAY" + time.Now().Format("20060102") + fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	tradeNo := "MOCK" + time.Now().Format("20060102150405") + fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	var paymentID int64
	err := s.svcCtx.DB.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		result, err := session.ExecCtx(ctx, `INSERT INTO payment(payment_no,idempotency_key,user_id,order_type,order_no,amount,channel,status,trade_no,create_time) VALUES(?,?,?,?,?,?,?,'pending','',NOW())`, paymentNo, idempotencyKey, userID, orderType, orderNo, amount, "mock")
		if err != nil {
			return err
		}
		paymentID, err = result.LastInsertId()
		if err != nil {
			return err
		}
		if _, err := session.ExecCtx(ctx, `INSERT INTO payment_log(payment_id,action,request,response,create_time) VALUES(?,?,?,?,NOW())`, paymentID, "create", fmt.Sprintf("orderType=%s,orderNo=%s,amount=%.2f,provider=mock", orderType, orderNo, amount), model.PaymentStatusPending); err != nil {
			return err
		}
		if _, err := session.ExecCtx(ctx, `UPDATE payment SET status='success',trade_no=? WHERE id=? AND status='pending'`, tradeNo, paymentID); err != nil {
			return err
		}
		_, err = session.ExecCtx(ctx, `INSERT INTO payment_log(payment_id,action,request,response,create_time) VALUES(?,?,?,?,NOW())`, paymentID, "mock_success", idempotencyKey, model.PaymentStatusSuccess)
		return err
	})
	if err != nil {
		if existing, findErr := s.svcCtx.PaymentModel.FindByIdempotencyKey(ctx, idempotencyKey); findErr == nil {
			return toRpc(existing), nil
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	paid, err := s.svcCtx.PaymentModel.FindOne(ctx, paymentID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if err := s.publishBookingSuccess(ctx, paid); err != nil {
		return nil, status.Error(codes.Unavailable, "payment notification unavailable")
	}
	return toRpc(paid), nil
}

func (s *PaymentBookingServer) publishBookingSuccess(ctx context.Context, paid *model.Payment) error {
	if paid == nil || paid.Status != model.PaymentStatusSuccess || s.publisher == nil {
		return nil
	}
	return s.publisher.Publish(ctx, mq.PaymentNotify{
		PaymentNo: paid.PaymentNo,
		UserId:    paid.UserId,
		OrderType: paid.OrderType,
		OrderNo:   paid.OrderNo,
		Amount:    paid.Amount,
		Action:    "success",
	})
}

func (s *PaymentBookingServer) GetOrderPayment(ctx context.Context, req *payment.GetOrderPaymentReq) (*payment.BookingPayment, error) {
	p, err := s.svcCtx.PaymentModel.FindByOrder(ctx, req.OrderType, req.OrderNo)
	if err != nil {
		if err == sqlx.ErrNotFound {
			return nil, status.Error(codes.NotFound, "payment not found")
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	return toRpc(p), nil
}

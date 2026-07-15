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
	svcCtx *svc.ServiceContext
}

func NewPaymentBookingServer(svcCtx *svc.ServiceContext) *PaymentBookingServer {
	return &PaymentBookingServer{svcCtx: svcCtx}
}

func toRpc(p *model.Payment) *payment.BookingPayment {
	return &payment.BookingPayment{Id: p.Id, PaymentNo: p.PaymentNo, OrderType: p.OrderType, OrderNo: p.OrderNo, Amount: p.Amount, Channel: p.Channel, Status: p.Status, TradeNo: p.TradeNo, Simulated: p.Channel == "mock"}
}

func (s *PaymentBookingServer) AutoPayBooking(ctx context.Context, req *payment.AutoPayBookingReq) (*payment.BookingPayment, error) {
	if s.svcCtx.Config.Provider != "mock" {
		return nil, status.Error(codes.FailedPrecondition, "mock payment disabled")
	}
	if req.OrderNo == "" || req.UserId == "" || req.Amount <= 0 || req.IdempotencyKey == "" {
		return nil, status.Error(codes.InvalidArgument, "invalid payment request")
	}
	if existing, err := s.svcCtx.PaymentModel.FindByIdempotencyKey(ctx, req.IdempotencyKey); err == nil {
		return toRpc(existing), nil
	}
	paymentNo := "PAY" + time.Now().Format("20060102") + fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	tradeNo := "MOCK" + time.Now().Format("20060102150405") + fmt.Sprintf("%06d", time.Now().UnixNano()%1000000)
	var paymentID int64
	err := s.svcCtx.DB.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		result, err := session.ExecCtx(ctx, `INSERT INTO payment(payment_no,idempotency_key,user_id,order_type,order_no,amount,channel,status,trade_no,create_time) VALUES(?,?,?,?,?,?,?,'pending','',NOW())`, paymentNo, req.IdempotencyKey, req.UserId, model.OrderTypeBooking, req.OrderNo, req.Amount, "mock")
		if err != nil {
			return err
		}
		paymentID, err = result.LastInsertId()
		if err != nil {
			return err
		}
		if _, err := session.ExecCtx(ctx, `INSERT INTO payment_log(payment_id,action,request,response,create_time) VALUES(?,?,?,?,NOW())`, paymentID, "create", fmt.Sprintf("booking=%s,amount=%.2f,provider=mock", req.OrderNo, req.Amount), model.PaymentStatusPending); err != nil {
			return err
		}
		if _, err := session.ExecCtx(ctx, `UPDATE payment SET status='success',trade_no=? WHERE id=? AND status='pending'`, tradeNo, paymentID); err != nil {
			return err
		}
		_, err = session.ExecCtx(ctx, `INSERT INTO payment_log(payment_id,action,request,response,create_time) VALUES(?,?,?,?,NOW())`, paymentID, "mock_success", req.IdempotencyKey, model.PaymentStatusSuccess)
		return err
	})
	if err != nil {
		if existing, findErr := s.svcCtx.PaymentModel.FindByIdempotencyKey(ctx, req.IdempotencyKey); findErr == nil {
			return toRpc(existing), nil
		}
		return nil, status.Error(codes.Internal, err.Error())
	}
	paid, err := s.svcCtx.PaymentModel.FindOne(ctx, paymentID)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}
	if s.svcCtx.MqProducer != nil {
		_ = s.svcCtx.MqProducer.Publish(ctx, mq.PaymentNotify{PaymentNo: paid.PaymentNo, UserId: paid.UserId, OrderType: paid.OrderType, OrderNo: paid.OrderNo, Amount: paid.Amount, Action: "success"})
	}
	return toRpc(paid), nil
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

package logic

import (
	"context"
	"errors"
	"testing"

	"github.com/askxuan/booking-service/internal/model"
	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/booking-service/internal/types"
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func serviceReaderContext(kind string, id int64, roles ...string) context.Context {
	ctx := context.WithValue(context.Background(), middleware.CtxKeyUserType, kind)
	ctx = context.WithValue(ctx, middleware.CtxKeyUserID, id)
	return context.WithValue(ctx, middleware.CtxKeyRoles, roles)
}

type serviceOrderReaderStub struct{ calls int }

func (r *serviceOrderReaderStub) Bookings(_ context.Context, q model.PlatformServiceOrderQuery) ([]*model.Booking, int64, error) {
	r.calls++
	return []*model.Booking{{Id: "B2", TempleId: "T2"}}, 21, nil
}
func (r *serviceOrderReaderStub) Consultations(_ context.Context, q model.PlatformServiceOrderQuery) ([]*model.Consultation, int64, error) {
	r.calls++
	return []*model.Consultation{{Id: "C2", TempleId: "T2", Question: "private order question", PaymentChannel: "demo_balance"}}, 21, nil
}

func TestPlatformServiceReaderRequiresHumanPlatformSuperBeforeAnyDataAccess(t *testing.T) {
	for _, tc := range []struct {
		name, kind string
		id         int64
		roles      []string
	}{
		{"anonymous", "", 0, nil}, {"temple", "admin", 2, []string{"temple_admin"}},
		{"master", "admin", 3, []string{"master"}}, {"customer", "user", 1, []string{"customer"}},
		{"custom-role", "admin", 4, []string{"finance_reader"}}, {"service", "service", 1, []string{"platform_super"}},
		{"forged-user-kind", "user", 1, []string{"platform_super"}}, {"missing-id", "admin", 0, []string{"platform_super"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := &serviceOrderReaderStub{}
			l := &PlatformServiceOrdersLogic{ctx: serviceReaderContext(tc.kind, tc.id, tc.roles...), reader: r}
			if _, err := l.Bookings(&types.PlatformServiceOrderListReq{}); err == nil || r.calls != 0 {
				t.Fatal("unauthorized list reached the reader")
			}
			if _, err := l.Consultations(&types.PlatformServiceOrderListReq{}); err == nil || r.calls != 0 {
				t.Fatal("unauthorized consultation list reached the reader")
			}
			if _, err := l.Booking("B2"); err == nil {
				t.Fatal("unauthorized detail accepted")
			}
			if _, err := l.Consultation("C2"); err == nil {
				t.Fatal("unauthorized consultation detail accepted")
			}
			if _, err := l.Fulfillment("B2"); err == nil {
				t.Fatal("unauthorized fulfillment accepted")
			}
		})
	}
}

func TestPlatformServiceReaderKeepsDomainsAndSensitiveQuestionSeparate(t *testing.T) {
	r := &serviceOrderReaderStub{}
	l := &PlatformServiceOrdersLogic{ctx: serviceReaderContext("admin", 9, "platform_super"), reader: r}
	b, err := l.Bookings(&types.PlatformServiceOrderListReq{Page: 2, Size: 20})
	if err != nil || b.Total != 21 || b.Page != 2 || len(b.List) != 1 || b.List[0].TempleId != "T2" {
		t.Fatalf("booking response incorrect: %v", err)
	}
	c, err := l.Consultations(&types.PlatformServiceOrderListReq{})
	if err != nil || c.Total != 21 || c.List[0].Question != "" || !c.List[0].Simulated {
		t.Fatal("list exposed a question or dropped the existing simulated-payment flag")
	}
	if _, err := l.Consultations(&types.PlatformServiceOrderListReq{Status: "confirmed"}); err == nil {
		t.Fatal("booking status accepted for a consultation")
	}
}

func TestPlatformServiceQueryRejectsInvalidDatesStatusesAndPagination(t *testing.T) {
	for _, req := range []types.PlatformServiceOrderListReq{
		{Page: -1}, {Size: 101}, {From: "2026-02-30"}, {From: "2026-10-23", To: "2026-10-22"},
		{Status: "active"}, {PaymentStatus: "made-up"}, {Q: string(make([]rune, 101))},
	} {
		if _, err := platformServiceQuery(&req, false); err == nil {
			t.Fatalf("invalid query accepted: %#v", req)
		}
	}
	q, err := platformServiceQuery(&types.PlatformServiceOrderListReq{Q: "  香  ", From: "2026-10-22", To: "2026-10-22"}, false)
	if err != nil || q.Q != "香" || q.Page != 1 || q.Size != 20 {
		t.Fatal("default page or inclusive date normalization failed")
	}
}

type serviceBookingLookup struct {
	model.BookingModel
	err error
}

func (b serviceBookingLookup) FindOne(context.Context, string) (*model.Booking, error) {
	return nil, b.err
}

type serviceConsultationLookup struct {
	model.ConsultationModel
	err error
}

func (c serviceConsultationLookup) FindOne(context.Context, string) (*model.Consultation, error) {
	return nil, c.err
}

func TestPlatformServiceDetailsDistinguishNotFoundFromReadFailure(t *testing.T) {
	ctx := serviceReaderContext("admin", 9, "platform_super")
	for _, tc := range []struct {
		err  error
		want error
	}{
		{sqlx.ErrNotFound, common.ErrBookingNotFound}, {errors.New("db offline"), common.ErrSystem},
	} {
		l := NewPlatformServiceOrdersLogic(ctx, &svc.ServiceContext{BookingModel: serviceBookingLookup{err: tc.err}})
		if _, err := l.Booking("B2"); err != tc.want {
			t.Fatalf("detail error=%v want=%v", err, tc.want)
		}
	}
	l := NewPlatformServiceOrdersLogic(ctx, &svc.ServiceContext{ConsultationModel: serviceConsultationLookup{err: sqlx.ErrNotFound}})
	if _, err := l.Consultation("C2"); err != common.ErrConsultationNotFound {
		t.Fatalf("consultation not found=%v", err)
	}
}

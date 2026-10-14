package logic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/askxuan/booking-service/internal/model"
	"github.com/askxuan/booking-service/internal/rpcclient"
	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/booking-service/internal/types"
	masterrpc "github.com/askxuan/booking-service/rpc/master"
	templerpc "github.com/askxuan/booking-service/rpc/temple"
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type bookingPathOrders struct {
	model.BookingModel
	saved    *model.Booking
	reserved bool
}

func (m *bookingPathOrders) FindByRequest(context.Context, string, string) (*model.Booking, error) {
	return nil, sqlx.ErrNotFound
}
func (m *bookingPathOrders) InsertWithReservation(_ context.Context, b *model.Booking, _ int) (*model.Booking, error) {
	m.saved = b
	m.reserved = true
	return b, nil
}
func (m *bookingPathOrders) InsertDirect(_ context.Context, b *model.Booking) (*model.Booking, error) {
	m.saved = b
	return b, nil
}

type bookingPathLogs struct{ model.BookingStatusLogModel }

func (bookingPathLogs) Insert(context.Context, *model.BookingStatusLog) error { return nil }

type bookingPathTemple struct{}

func (bookingPathTemple) GetBookingService(_ context.Context, temple, service string) (*templerpc.BookingService, error) {
	return &templerpc.BookingService{TempleCode: temple, TempleName: "Test temple", TempleStatus: "正常", ServiceCode: service, ServiceName: "Test service", ServiceStatus: "on_shelf", Price: 300, Slots: []*templerpc.BookingSlot{{Code: "AM", StartTime: "09:00", EndTime: "12:00", Capacity: 10, Status: "enabled"}}}, nil
}

type bookingPathMaster struct{ rpcclient.MasterClient }

func (bookingPathMaster) GetByCode(_ context.Context, code string) (*masterrpc.BookingMaster, error) {
	return &masterrpc.BookingMaster{Code: code, TempleCode: "T002", ShelfStatus: "on_shelf", PlatformStatus: "normal"}, nil
}

type bookingPathConsultation struct {
	model.ConsultationModel
	paid         bool
	calls        int
	err          error
	user, master string
}

func (m *bookingPathConsultation) HasValidPaidConsultation(_ context.Context, user, master string) (bool, error) {
	m.calls++
	m.user, m.master = user, master
	return m.paid, m.err
}

func bookingPathFixture(t *testing.T, manageBy, tagStatus string, httpStatus int) (context.Context, *svc.ServiceContext, *bookingPathOrders, *bookingPathConsultation) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if httpStatus != http.StatusOK {
			w.WriteHeader(httpStatus)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": 0, "data": map[string]interface{}{"id": "M002", "templeId": "T002", "manageBy": manageBy, "serviceTags": []map[string]interface{}{{"serviceCode": "S007", "price": 49, "status": tagStatus}}}})
	}))
	t.Cleanup(server.Close)
	orders := &bookingPathOrders{}
	consult := &bookingPathConsultation{}
	s := &svc.ServiceContext{BookingModel: orders, StatusLogModel: bookingPathLogs{}, TempleClient: bookingPathTemple{}, MasterClient: bookingPathMaster{}, ConsultationModel: consult}
	s.Config.GatewayBaseURL = server.URL
	return context.WithValue(context.Background(), middleware.CtxKeyUserID, int64(123)), s, orders, consult
}
func bookingPathReq(service string) *types.CreateReq {
	return &types.CreateReq{RequestId: "isolated-test", TempleId: "T002", MasterId: "M002", ServiceId: service, BookingDate: time.Now().Add(24 * time.Hour).Format("2006-01-02"), SlotCode: "AM", ManualPayment: true}
}
func assertBookingCode(t *testing.T, err error, code int) {
	t.Helper()
	var business *common.BizError
	if !errors.As(err, &business) || business.Code != code {
		t.Fatalf("expected code %d, got %v", code, err)
	}
}

func TestTempleMasterBookingRejectsUnsupportedServices(t *testing.T) {
	for _, tt := range []struct {
		name, service, status, manageBy string
		httpStatus, wantCode            int
	}{
		{"unconfigured service", "S001", "enabled", "temple", 200, 40311},
		{"disabled service", "S007", "disabled", "temple", 200, 40311},
		{"dependency failure never falls back to temple price", "S007", "enabled", "temple", 503, 50205},
		{"platform master cannot use temple path", "S007", "enabled", "platform", 200, 40312},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, s, orders, _ := bookingPathFixture(t, tt.manageBy, tt.status, tt.httpStatus)
			_, err := NewCreateLogic(ctx, s).Create(bookingPathReq(tt.service))
			assertBookingCode(t, err, tt.wantCode)
			if orders.saved != nil {
				t.Fatal("invalid request created an order")
			}
		})
	}
}
func TestTempleMasterBookingUsesCompletePriceAndTempleInventory(t *testing.T) {
	ctx, s, orders, consult := bookingPathFixture(t, "temple", "enabled", 200)
	req := bookingPathReq("S007")
	req.MeritMoney = 20
	consult.paid = true
	result, err := NewCreateLogic(ctx, s).Create(req)
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalFee != 369 || orders.saved.ServiceFee != 49 || orders.saved.MeritMoney != 320 || !orders.reserved {
		t.Fatalf("incorrect split or inventory: %+v", orders.saved)
	}
	if consult.calls != 1 {
		t.Fatal("temple booking must require a valid paid consultation")
	}
}
func TestWholeTempleBookingDoesNotRequireMasterTags(t *testing.T) {
	ctx, s, orders, _ := bookingPathFixture(t, "temple", "disabled", 503)
	req := bookingPathReq("S001")
	req.MasterId = ""
	result, err := NewCreateLogic(ctx, s).Create(req)
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalFee != 300 || orders.saved.MasterId != "" || orders.saved.ServiceFee != 0 || !orders.reserved {
		t.Fatalf("whole temple booking changed: %+v", orders.saved)
	}
}
func TestDirectBookingRejectsTempleMasterAndRetainsConsultationGate(t *testing.T) {
	for _, tt := range []struct {
		name, manageBy string
		paid           bool
		wantCode       int
	}{
		{"temple master cannot bypass temple fee", "temple", true, 40312},
		{"independent master requires prior consultation", "platform", false, 40415},
		{"independent master after consultation", "platform", true, 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ctx, s, orders, consult := bookingPathFixture(t, tt.manageBy, "enabled", 200)
			consult.paid = tt.paid
			result, err := NewDirectBookingLogic(ctx, s).Create("M002", &types.DirectBookingReq{RequestId: "isolated-direct", ServiceCode: "S007", BookingDate: time.Now().Add(24 * time.Hour).Format("2006-01-02"), ManualPayment: true})
			if tt.wantCode != 0 {
				assertBookingCode(t, err, tt.wantCode)
				if orders.saved != nil {
					t.Fatal("invalid request created an order")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if result.TotalFee != 49 || orders.reserved || orders.saved.TimeSlot != "待协商" {
				t.Fatalf("incorrect direct booking: %+v", orders.saved)
			}
		})
	}
}

func (m *bookingPathOrders) FindOne(_ context.Context, id string) (*model.Booking, error) {
	return m.saved, nil
}

func TestNamedMasterRequiresValidConsultationOnBothPaths(t *testing.T) {
	for _, track := range []string{"temple", "platform"} {
		for _, dependencyFailure := range []bool{false, true} {
			t.Run(track+fmt.Sprint(dependencyFailure), func(t *testing.T) {
				ctx, s, orders, consult := bookingPathFixture(t, track, "enabled", 200)
				if dependencyFailure {
					consult.err = errors.New("unavailable")
				}
				var err error
				if track == "temple" {
					_, err = NewCreateLogic(ctx, s).Create(bookingPathReq("S007"))
				} else {
					_, err = NewDirectBookingLogic(ctx, s).Create("M002", &types.DirectBookingReq{ServiceCode: "S007", BookingDate: time.Now().Add(24 * time.Hour).Format("2006-01-02"), RequestId: "test-prerequisite", ManualPayment: true})
				}
				if err == nil {
					t.Fatal("invalid consultation allowed a booking")
				}
				if !dependencyFailure {
					assertBookingCode(t, err, 40415)
				}
				if orders.saved != nil || orders.reserved {
					t.Fatal("ineligible request changed inventory/orders")
				}
				if consult.user != "123" || consult.master != "M002" {
					t.Fatalf("wrong prerequisite identity: %+v", consult)
				}
			})
		}
	}
}
func TestPendingBookingCannotPayAfterConsultationExpires(t *testing.T) {
	ctx, s, orders, consult := bookingPathFixture(t, "temple", "enabled", 200)
	orders.saved = &model.Booking{Id: "existing", UserId: "123", MasterId: "M002", Status: model.StatusPendingPayment, PaymentStatus: "pending", PaymentExpireTime: time.Now().Add(time.Hour).Format("2006-01-02 15:04:05")}
	_, err := NewPayLogic(ctx, s).Pay(&types.PayReq{Id: "existing"})
	assertBookingCode(t, err, 40415)
	if consult.calls != 1 {
		t.Fatal("pending order skipped prerequisite")
	}
	orders.saved.PaymentStatus = model.PaymentStatusSuccess
	if _, err = NewPayLogic(ctx, s).Pay(&types.PayReq{Id: "existing"}); err != nil {
		t.Fatalf("already paid order must remain readable: %v", err)
	}
	if consult.calls != 1 {
		t.Fatal("already paid result should not recheck consultation")
	}
}

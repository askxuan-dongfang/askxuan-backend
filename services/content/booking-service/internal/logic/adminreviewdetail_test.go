package logic

import (
	"context"
	"github.com/askxuan/common/middleware"
	"testing"

	"github.com/askxuan/booking-service/internal/model"
	"github.com/askxuan/booking-service/internal/svc"
	"github.com/askxuan/booking-service/internal/types"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

type adminReviewModelStub struct {
	review *model.BookingReview
	err    error
}

func (s *adminReviewModelStub) Insert(context.Context, *model.BookingReview) (*model.BookingReview, error) {
	panic("not used")
}

func (s *adminReviewModelStub) FindOne(context.Context, string) (*model.BookingReview, error) {
	return s.review, s.err
}

func (s *adminReviewModelStub) UpdateReply(context.Context, string, string) (*model.BookingReview, error) {
	panic("not used")
}

func TestAdminReviewDetail(t *testing.T) {
	logic := NewAdminReviewDetailLogic(reviewAdminContext(), &svc.ServiceContext{
		BookingModel: reviewAdminBookings{},
		ReviewModel: &adminReviewModelStub{review: &model.BookingReview{
			Id: 7, BookingId: "B001", UserId: "U001", Rating: 5, Content: "庄重圆满",
		}},
	})

	got, err := logic.AdminReviewDetail(&types.ReviewDetailReq{Id: "B001"})
	if err != nil {
		t.Fatalf("AdminReviewDetail() error = %v", err)
	}
	if got.BookingId != "B001" || got.Rating != 5 || got.Content != "庄重圆满" {
		t.Fatalf("AdminReviewDetail() = %#v", got)
	}
}

func TestAdminReviewDetailNotFound(t *testing.T) {
	logic := NewAdminReviewDetailLogic(reviewAdminContext(), &svc.ServiceContext{
		BookingModel: reviewAdminBookings{},
		ReviewModel:  &adminReviewModelStub{err: sqlx.ErrNotFound},
	})

	_, err := logic.AdminReviewDetail(&types.ReviewDetailReq{Id: "missing"})
	if err != common.ErrReviewNotFound {
		t.Fatalf("AdminReviewDetail() error = %v, want %v", err, common.ErrReviewNotFound)
	}
}

type reviewAdminBookings struct{ model.BookingModel }

func (reviewAdminBookings) FindOne(ctx context.Context, id string) (*model.Booking, error) {
	return &model.Booking{Id: id, TempleId: "TTEST"}, nil
}
func reviewAdminContext() context.Context {
	ctx := context.WithValue(context.Background(), middleware.CtxKeyUserID, int64(7))
	ctx = context.WithValue(ctx, middleware.CtxKeyRoles, []string{"temple_admin"})
	return context.WithValue(ctx, middleware.CtxKeyTempleCode, "TTEST")
}
func TestAdminReviewDetailCrossTemple(t *testing.T) {
	ctx := context.WithValue(reviewAdminContext(), middleware.CtxKeyTempleCode, "OTHER")
	_, err := NewAdminReviewDetailLogic(ctx, &svc.ServiceContext{BookingModel: reviewAdminBookings{}}).AdminReviewDetail(&types.ReviewDetailReq{Id: "B001"})
	if err != common.ErrForbidden {
		t.Fatalf("expected forbidden: %v", err)
	}
}

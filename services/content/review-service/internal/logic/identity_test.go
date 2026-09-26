package logic

import (
	"context"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/review-service/internal/svc"
	"github.com/askxuan/review-service/internal/types"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestMasterIdentityUsesProfileCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/admin/masters/profile" || r.Header.Get("Authorization") != "Bearer test" {
			t.Error("identity not forwarded")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"data":{"id":"W003"}}`))
	}))
	defer server.Close()
	t.Setenv("ASKXUAN_GATEWAY_URL", server.URL)
	ctx := WithAuthorization(context.WithValue(context.Background(), middleware.CtxKeyMasterID, int64(31)), "Bearer test")
	code, err := masterCode(ctx)
	if err != nil || code != "W003" {
		t.Fatalf("code=%s err=%v", code, err)
	}
}
func TestCommerceReviewEligibility(t *testing.T) {
	cases := []struct {
		name, user, status string
		code               int
	}{{"other-owner", "2", "completed", 0}, {"unfinished", "1", "paid", 0}, {"missing", "", "", 40401}}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"code":` + strconv.Itoa(tc.code) + `,"data":{"userId":"` + tc.user + `","status":"` + tc.status + `"}}`))
			}))
			defer server.Close()
			t.Setenv("ASKXUAN_GATEWAY_URL", server.URL)
			ctx := WithAuthorization(context.WithValue(context.Background(), middleware.CtxKeyUserID, int64(1)), "Bearer test")
			_, err := NewCreateReviewLogic(ctx, &svc.ServiceContext{}).CreateReview(&types.CreateReviewReq{UserId: "1", TargetType: "shop_order", TargetId: "42", Rating: 5, Content: "Test", Images: "[]"})
			if err == nil {
				t.Fatal("ineligible review accepted")
			}
		})
	}
}

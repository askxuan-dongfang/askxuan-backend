package handler

import (
	"encoding/json"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/askxuan/common"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/payment-service/internal/wallet"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// Local browser fixture uses the real wallet SQL projection. Never started without an explicit test flag.
func TestWalletBrowserFixture(t *testing.T) {
	if os.Getenv("WALLET_BROWSER_FIXTURE") != "1" {
		t.Skip("local browser fixture disabled")
	}
	auth := &middleware.AuthConfig{Secret: "local-wallet-fixture-only"}
	role := &middleware.AdminAuthConfig{AllowedRoles: []string{"customer"}}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/payments/wallet", auth.AuthFunc(role.AdminAuthFunc(walletHandler(wallet.Store{DB: sqlx.NewMysql(os.Getenv("WALLET_TEST_DSN"))}))))
	t.Fatal(http.ListenAndServe("127.0.0.1:18191", mux))
}

func TestWalletRejectsUnauthenticatedAndNonCustomer(t *testing.T) {
	auth := &middleware.AuthConfig{Secret: "wallet-test"}
	role := &middleware.AdminAuthConfig{AllowedRoles: []string{"customer"}}
	h := auth.AuthFunc(role.AdminAuthFunc(walletHandler(wallet.Store{})))
	master, _ := common.GenAccessToken("wallet-test", common.TokenInfo{UserId: 1, UserType: "admin", Roles: []string{"master"}, MasterID: 1}, 60)
	for _, token := range []string{"", master} {
		req := httptest.NewRequest("GET", "/api/v1/payments/wallet?userId=1", nil)
		req.Header.Set("X-User-Id", "1")
		req.Header.Set("X-Roles", "customer")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		w := httptest.NewRecorder()
		h(w, req)
		var body struct {
			Code int `json:"code"`
		}
		if json.Unmarshal(w.Body.Bytes(), &body) != nil || body.Code == 0 {
			t.Fatal("unauthorized wallet access")
		}
	}
}
func TestWalletOwnerCannotBeOverriddenByQueryOrHeaders(t *testing.T) {
	raw, mock, e := sqlmock.New()
	if e != nil {
		t.Fatal(e)
	}
	defer raw.Close()
	mock.ExpectQuery("SELECT COALESCE.*FROM payment").WithArgs("91001").WillReturnRows(sqlmock.NewRows([]string{"paid"}).AddRow(0))
	mock.ExpectQuery("SELECT COALESCE.*FROM refund").WithArgs("91001").WillReturnRows(sqlmock.NewRows([]string{"refunded", "refunding"}).AddRow(0, 0))
	mock.ExpectQuery("SELECT COUNT").WithArgs("91001").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT p.id").WithArgs("91001", 0).WillReturnRows(sqlmock.NewRows([]string{"id", "payment_no", "order_type", "order_no", "amount", "channel", "status", "created_at"}))
	auth := &middleware.AuthConfig{Secret: "wallet-test"}
	role := &middleware.AdminAuthConfig{AllowedRoles: []string{"customer"}}
	token, _ := common.GenAccessToken("wallet-test", common.TokenInfo{UserId: 91001, UserType: "user", Roles: []string{"customer"}}, 60)
	req := httptest.NewRequest("GET", "/api/v1/payments/wallet?userId=91002&mode=mock", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-User-Id", "91002")
	w := httptest.NewRecorder()
	auth.AuthFunc(role.AdminAuthFunc(walletHandler(wallet.Store{DB: sqlx.NewSqlConnFromDB(raw)})))(w, req)
	var body struct {
		Code int `json:"code"`
	}
	if json.Unmarshal(w.Body.Bytes(), &body) != nil || body.Code != 0 {
		t.Fatalf("wallet failed: %s", w.Body.String())
	}
	if e := mock.ExpectationsWereMet(); e != nil {
		t.Fatal(e)
	}
}

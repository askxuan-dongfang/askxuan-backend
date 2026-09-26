package handler

import (
	"context"
	"github.com/askxuan/common/middleware"
	"github.com/askxuan/finance-service/internal/model"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"net/http"
	"os"
	"testing"
)

func TestWalletOwnerRequiresMatchingRoleAndEntity(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.CtxKeyRoles, []string{"master"})
	ctx = context.WithValue(ctx, middleware.CtxKeyMasterID, int64(901))
	ctx = context.WithValue(ctx, middleware.CtxKeyTempleCode, "T901")
	if id, ok := walletOwner(ctx, "master"); !ok || id != "901" {
		t.Fatal("master owner unresolved")
	}
	if _, ok := walletOwner(ctx, "temple"); ok {
		t.Fatal("managed master gained temple wallet")
	}
	ctx = context.WithValue(ctx, middleware.CtxKeyRoles, []string{"temple_admin"})
	if id, ok := walletOwner(ctx, "temple"); !ok || id != "T901" {
		t.Fatal("temple owner unresolved")
	}
	if _, ok := walletOwner(ctx, "master"); ok {
		t.Fatal("temple admin gained master wallet")
	}
	if _, ok := walletOwner(context.Background(), "master"); ok {
		t.Fatal("guest accepted")
	}
	ctx = context.WithValue(ctx, middleware.CtxKeyTempleCode, "")
	if _, ok := walletOwner(ctx, "temple"); ok {
		t.Fatal("missing identity accepted")
	}
}

func TestWalletBrowserFixture(t *testing.T) {
	if os.Getenv("WALLET_BROWSER_FIXTURE") != "1" {
		t.Skip("local browser fixture disabled")
	}
	model.Configure(sqlx.NewMysql(os.Getenv("WALLET_TEST_DSN")))
	auth := &middleware.AuthConfig{Secret: "local-wallet-fixture-only"}
	mux := http.NewServeMux()
	for _, kind := range []string{"master", "temple"} {
		mux.HandleFunc("/api/v1/finance/wallet/"+kind, auth.AuthFunc(providerWalletHandler(kind)))
	}
	t.Fatal(http.ListenAndServe("127.0.0.1:18192", mux))
}

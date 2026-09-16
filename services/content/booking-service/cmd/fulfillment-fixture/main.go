// Local-only fixture: real booking handlers and MySQL, synthetic identities and master lookup.
package main

import (
	"context"
	"encoding/json"
	"github.com/askxuan/booking-service/internal/handler"
	"github.com/askxuan/booking-service/internal/model"
	"github.com/askxuan/booking-service/internal/svc"
	masterrpc "github.com/askxuan/booking-service/rpc/master"
	"github.com/askxuan/common"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"github.com/zeromicro/go-zero/rest"
	"os"
	"strings"
)

type masters struct{}

func (masters) GetByID(_ context.Context, id int64) (*masterrpc.BookingMaster, error) {
	return &masterrpc.BookingMaster{Id: id, Code: "CHATTEST", DharmaName: "测试大师"}, nil
}
func (masters) GetByCode(_ context.Context, code string) (*masterrpc.BookingMaster, error) {
	return &masterrpc.BookingMaster{Id: 98765002, Code: code, DharmaName: "测试大师"}, nil
}
func main() {
	dsn := os.Getenv("FULFILLMENT_TEST_DSN")
	if !strings.Contains(dsn, "@tcp(127.0.0.1:53306)/askxuan_fulfillment_test_browser?") {
		panic("requires isolated local fixture")
	}
	sqlx.DisableLog()
	db := sqlx.NewMysql(dsn)
	const secret = "local-fulfillment-fixture-only-not-production"
	s := &svc.ServiceContext{DB: db, BookingModel: model.NewBookingModel(db), StatusLogModel: model.NewBookingStatusLogModel(db), ChatModel: model.NewBookingChatModel(db), ReviewModel: model.NewBookingReviewModel(db), MasterClient: masters{}}
	s.Config.AuthSecret = secret
	identities := map[string]common.TokenInfo{"customer": {UserId: 98765001, UserType: "user", Roles: []string{"customer"}, ClientID: "customer"}, "temple": {UserId: 7, UserType: "admin", Roles: []string{"temple_admin"}, ClientID: "temple-admin", TempleID: 1, TempleCode: "TTEST"}, "master": {UserId: 6, UserType: "admin", Roles: []string{"master"}, ClientID: "master", MasterID: 98765002}}
	tokens := map[string]string{}
	for role, info := range identities {
		token, e := common.GenAccessToken(secret, info, 7200)
		if e != nil {
			panic(e)
		}
		tokens[role] = token
	}
	payload, _ := json.Marshal(tokens)
	if e := os.WriteFile(os.Getenv("FULFILLMENT_FIXTURE_TOKENS"), payload, 0600); e != nil {
		panic(e)
	}
	var c rest.RestConf
	err := conf.LoadFromYamlBytes([]byte("Name: fulfillment-fixture\nHost: 127.0.0.1\nPort: 58085\nTimeout: 120000\n"), &c)
	if err != nil {
		panic(err)
	}
	server := rest.MustNewServer(c)
	defer server.Stop()
	handler.RegisterHandlers(server, s)
	server.Start()
}

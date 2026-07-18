package rpcclient

import (
	"context"
	"time"

	"github.com/askxuan/common/rpc/shoporder"
	"github.com/zeromicro/go-zero/zrpc"
)

type ShopOrderClient interface {
	ValidatePayment(ctx context.Context, orderNo, userID string, amount float64) (*shoporder.ValidatedShopOrder, error)
}

type shopOrderClient struct {
	client shoporder.ShopOrderServiceClient
}

func NewShopOrderClient(c zrpc.Client) ShopOrderClient {
	return &shopOrderClient{client: shoporder.NewShopOrderServiceClient(c.Conn())}
}

func (c *shopOrderClient) ValidatePayment(ctx context.Context, orderNo, userID string, amount float64) (*shoporder.ValidatedShopOrder, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return c.client.ValidatePayment(ctx, &shoporder.ValidatePaymentReq{OrderNo: orderNo, UserId: userID, Amount: amount})
}

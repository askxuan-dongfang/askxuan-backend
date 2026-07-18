package rpcclient

import (
	"context"
	"time"

	"github.com/askxuan/common/rpc/catalog"
	"github.com/zeromicro/go-zero/zrpc"
)

type CatalogClient interface {
	ReserveCart(ctx context.Context, requestID string, items []*catalog.CartLine) (*catalog.ReserveCartResp, error)
	ReleaseCart(ctx context.Context, requestID string) error
}

type catalogClient struct{ client catalog.CatalogServiceClient }

func NewCatalogClient(c zrpc.Client) CatalogClient {
	return &catalogClient{client: catalog.NewCatalogServiceClient(c.Conn())}
}

func (c *catalogClient) ReserveCart(ctx context.Context, requestID string, items []*catalog.CartLine) (*catalog.ReserveCartResp, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return c.client.ReserveCart(ctx, &catalog.ReserveCartReq{RequestId: requestID, Items: items})
}

func (c *catalogClient) ReleaseCart(ctx context.Context, requestID string) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, err := c.client.ReleaseCart(ctx, &catalog.ReleaseCartReq{RequestId: requestID})
	return err
}

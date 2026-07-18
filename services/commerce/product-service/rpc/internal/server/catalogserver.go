package server

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/askxuan/common/rpc/catalog"
	"github.com/askxuan/product-service/internal/model"
	"github.com/askxuan/product-service/internal/svc"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CatalogServer struct {
	catalog.UnimplementedCatalogServiceServer
	svcCtx *svc.ServiceContext
}

type reservationRecord struct {
	Status   string `db:"status"`
	Snapshot string `db:"snapshot"`
}

func NewCatalogServer(svcCtx *svc.ServiceContext) *CatalogServer {
	return &CatalogServer{svcCtx: svcCtx}
}

func (s *CatalogServer) ReserveCart(ctx context.Context, req *catalog.ReserveCartReq) (*catalog.ReserveCartResp, error) {
	if req.GetRequestId() == "" || len(req.GetItems()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "requestId 和商品不能为空")
	}
	if existing, err := s.loadReservation(ctx, req.GetRequestId()); err == nil {
		return existing, nil
	}

	resp := &catalog.ReserveCartResp{RequestId: req.GetRequestId()}
	err := s.svcCtx.DB.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		for _, line := range req.GetItems() {
			if line.GetProductId() <= 0 || line.GetQuantity() <= 0 {
				return status.Error(codes.InvalidArgument, "商品或数量无效")
			}
			var product model.Product
			if err := session.QueryRowCtx(ctx, &product, `SELECT id,product_no,name,category_id,description,main_image,status,price,market_price,stock,tags,freight_template_id,create_time,update_time FROM product WHERE id=? FOR UPDATE`, line.GetProductId()); err != nil {
				if errors.Is(err, sqlx.ErrNotFound) {
					return status.Error(codes.NotFound, "商品不存在")
				}
				return err
			}
			if product.Status != model.ProductStatusOnShelf {
				return status.Error(codes.FailedPrecondition, "商品已下架")
			}

			quote := &catalog.CatalogQuote{ProductId: product.Id, ProductName: product.Name,
				Quantity: line.GetQuantity(), Image: product.MainImage, UnitPrice: product.Price}
			if line.GetSkuId() > 0 {
				var sku model.ProductSku
				if err := session.QueryRowCtx(ctx, &sku, `SELECT id,product_id,spec_name,spec_value,price,stock,sku_no FROM product_sku WHERE id=? AND product_id=? FOR UPDATE`, line.GetSkuId(), product.Id); err != nil {
					return status.Error(codes.NotFound, "商品规格不存在")
				}
				if sku.Stock < int(line.GetQuantity()) {
					return status.Error(codes.ResourceExhausted, "商品规格库存不足")
				}
				if _, err := session.ExecCtx(ctx, `UPDATE product_sku SET stock=stock-? WHERE id=? AND stock>=?`, line.GetQuantity(), sku.Id, line.GetQuantity()); err != nil {
					return err
				}
				quote.SkuId, quote.UnitPrice = sku.Id, sku.Price
				quote.SkuSpec = sku.SpecName + "：" + sku.SpecValue
			} else {
				if product.Stock < int(line.GetQuantity()) {
					return status.Error(codes.ResourceExhausted, "商品库存不足")
				}
				if _, err := session.ExecCtx(ctx, `UPDATE product SET stock=stock-? WHERE id=? AND stock>=?`, line.GetQuantity(), product.Id, line.GetQuantity()); err != nil {
					return err
				}
				quote.SkuSpec = "默认规格"
			}
			resp.Items = append(resp.Items, quote)
			resp.TotalAmount += quote.UnitPrice * float64(quote.Quantity)
		}
		snapshot, err := json.Marshal(resp)
		if err != nil {
			return err
		}
		_, err = session.ExecCtx(ctx, `INSERT INTO product_stock_reservation(request_id,status,snapshot,create_time,update_time) VALUES(?,'reserved',?,NOW(),NOW())`, req.GetRequestId(), string(snapshot))
		return err
	})
	if err != nil {
		if existing, loadErr := s.loadReservation(ctx, req.GetRequestId()); loadErr == nil {
			return existing, nil
		}
		return nil, err
	}
	return resp, nil
}

func (s *CatalogServer) ReleaseCart(ctx context.Context, req *catalog.ReleaseCartReq) (*catalog.ReleaseCartResp, error) {
	if req.GetRequestId() == "" {
		return nil, status.Error(codes.InvalidArgument, "requestId 不能为空")
	}
	released := false
	err := s.svcCtx.DB.TransactCtx(ctx, func(ctx context.Context, session sqlx.Session) error {
		var record reservationRecord
		if err := session.QueryRowCtx(ctx, &record, `SELECT status,snapshot FROM product_stock_reservation WHERE request_id=? FOR UPDATE`, req.GetRequestId()); err != nil {
			if errors.Is(err, sqlx.ErrNotFound) {
				return nil
			}
			return err
		}
		if record.Status != "reserved" {
			return nil
		}
		var snapshot catalog.ReserveCartResp
		if err := json.Unmarshal([]byte(record.Snapshot), &snapshot); err != nil {
			return err
		}
		for _, item := range snapshot.Items {
			if item.SkuId > 0 {
				if _, err := session.ExecCtx(ctx, `UPDATE product_sku SET stock=stock+? WHERE id=?`, item.Quantity, item.SkuId); err != nil {
					return err
				}
			} else if _, err := session.ExecCtx(ctx, `UPDATE product SET stock=stock+? WHERE id=?`, item.Quantity, item.ProductId); err != nil {
				return err
			}
		}
		if _, err := session.ExecCtx(ctx, `UPDATE product_stock_reservation SET status='released',update_time=NOW() WHERE request_id=?`, req.GetRequestId()); err != nil {
			return err
		}
		released = true
		return nil
	})
	return &catalog.ReleaseCartResp{Released: released}, err
}

func (s *CatalogServer) loadReservation(ctx context.Context, requestID string) (*catalog.ReserveCartResp, error) {
	var record reservationRecord
	if err := s.svcCtx.DB.QueryRowCtx(ctx, &record, `SELECT status,snapshot FROM product_stock_reservation WHERE request_id=?`, requestID); err != nil {
		return nil, err
	}
	if record.Status != "reserved" {
		return nil, sqlx.ErrNotFound
	}
	var resp catalog.ReserveCartResp
	if err := json.Unmarshal([]byte(record.Snapshot), &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

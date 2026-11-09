package logic

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/askxuan/common"
	"github.com/askxuan/temple-service/internal/model"
	"github.com/askxuan/temple-service/internal/svc"
	"github.com/askxuan/temple-service/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

type updatePriceTemple struct{ model.TempleModel }

func (updatePriceTemple) FindOneByPk(context.Context, int64) (*model.Temple, error) {
	return &model.Temple{Id: 1, Code: "T001"}, nil
}

type updatePriceCatalog struct{ model.ServiceTypeModel }

func (updatePriceCatalog) FindOne(context.Context, string) (*model.ServiceType, error) {
	return &model.ServiceType{Code: "S001", Name: "祈福"}, nil
}

type updatePriceService struct {
	model.TempleServiceModel
	saved *model.TempleServiceRecord
}

func (*updatePriceService) FindOne(context.Context, int64) (*model.TempleServiceRecord, error) {
	return &model.TempleServiceRecord{Id: 1, TempleCode: "T001", ServiceCode: "S001", Price: 88}, nil
}
func (m *updatePriceService) Update(_ context.Context, record *model.TempleServiceRecord) error {
	m.saved = record
	return nil
}
func (*updatePriceService) FindIntentTags(context.Context, int64) ([]string, error) {
	return []string{}, nil
}

func TestAdminServiceUpdatePricePresence(t *testing.T) {
	for _, tt := range []struct {
		name, body string
		want       float64
		invalid    bool
	}{
		{name: "omitted preserves prior price", body: `{}`, want: 88},
		{name: "zero is an explicit free price", body: `{"price":0}`, want: 0},
		{name: "positive decimal is preserved", body: `{"price":9.90}`, want: 9.9},
		{name: "negative is rejected before writing", body: `{"price":-1}`, invalid: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("PUT", "/services/1", strings.NewReader(tt.body))
			r.Header.Set("Content-Type", "application/json")
			var req types.TempleServiceUpdateReq
			if err := httpx.ParseJsonBody(r, &req); err != nil {
				t.Fatal(err)
			}
			req.Id = 1
			service := &updatePriceService{}
			l := NewAdminServiceUpdateLogic(svc.WithTempleID(context.Background(), 1), &svc.ServiceContext{
				TempleModel: updatePriceTemple{}, ServiceTypeModel: updatePriceCatalog{}, TempleServiceModel: service,
			})
			resp, err := l.AdminServiceUpdate(&req)
			if tt.invalid {
				if !errors.Is(err, common.ErrParamInvalid) || service.saved != nil {
					t.Fatalf("negative price: err=%v saved=%v", err, service.saved)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if service.saved == nil || service.saved.Price != tt.want || resp.Price != tt.want {
				t.Fatalf("want saved and returned price %v, saved=%v response=%v", tt.want, service.saved, resp.Price)
			}
		})
	}
}

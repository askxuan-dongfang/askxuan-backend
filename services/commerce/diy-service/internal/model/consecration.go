package model

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// Consecration is a reference to an existing S006 offering, never a DIY markup.
// The immutable order snapshot preserves its provider and price for fulfillment.
type Consecration struct {
	Id                int64   `db:"id" json:"id"`
	ServiceCode       string  `db:"service_code" json:"serviceCode"`
	ServiceName       string  `db:"service_name" json:"serviceName"`
	SourceServiceId   int64   `db:"source_service_id" json:"sourceServiceId"`
	SourceServiceCode string  `db:"source_service_code" json:"sourceServiceCode"`
	ProviderType      string  `db:"provider_type" json:"providerType"`
	TempleCode        string  `db:"temple_code" json:"templeCode"`
	TempleName        string  `db:"temple_name" json:"templeName"`
	MasterCode        string  `db:"master_code" json:"masterCode"`
	MasterName        string  `db:"master_name" json:"masterName"`
	Price             float64 `db:"price" json:"price"`
}

// Temple staff are dispatched by their temple. Independently practising masters
// offer their own service; listing temple staff separately would omit temple fees.
const consecrationCatalog = `(
 SELECT s.id, CONCAT('consecration:temple:',s.id) service_code, '开光' service_name,
 s.id source_service_id, s.service_code source_service_code, 'temple' provider_type,
 t.code temple_code, t.name temple_name, '' master_code, '' master_name, s.price
 FROM askxuan_temple.temple_service s JOIN askxuan_temple.temple t ON t.code=s.temple_code
 WHERE s.service_code='S006' AND s.status='on_shelf' AND s.price>=0 AND t.status IN ('正常','推荐') AND t.auth_status='certified' AND t.platform_status='normal' AND t.shelf_status='on_shelf'
 UNION ALL
 SELECT -s.id, CONCAT('consecration:master:',s.id), '开光', s.id, s.service_code, 'master',
 '', '', m.code, m.dharma_name, s.price
 FROM askxuan_master.master_service_tag s JOIN askxuan_master.master m ON m.code=s.master_code
 WHERE s.service_code='S006' AND s.status='enabled' AND s.price>=0
 AND m.manage_by='platform' AND m.shelf_status='on_shelf' AND m.platform_status='normal' AND m.auth_status='已认证'
) consecration`

func ListConsecrations(ctx context.Context, conn sqlx.SqlConn, page, size int) ([]Consecration, int64, error) {
	var total int64
	if err := conn.QueryRowCtx(ctx, &total, "SELECT COUNT(*) FROM "+consecrationCatalog); err != nil {
		return nil, 0, err
	}
	rows := make([]Consecration, 0)
	err := conn.QueryRowsCtx(ctx, &rows, "SELECT * FROM "+consecrationCatalog+" ORDER BY provider_type DESC, source_service_id LIMIT ? OFFSET ?", size, (page-1)*size)
	return rows, total, err
}

func ValidConsecrationCode(code string) bool {
	parts := strings.Split(code, ":")
	if len(parts) != 3 || parts[0] != "consecration" || (parts[1] != "temple" && parts[1] != "master") {
		return false
	}
	id, err := strconv.ParseInt(parts[2], 10, 64)
	return err == nil && id > 0 && strconv.FormatInt(id, 10) == parts[2]
}

func resolveConsecration(ctx context.Context, session sqlx.Session, code string) (*Consecration, error) {
	if code == "" {
		return nil, nil
	}
	if !ValidConsecrationCode(code) {
		return nil, ErrOrderBlessingUnavailable
	}
	var service Consecration
	if err := session.QueryRowCtx(ctx, &service, "SELECT * FROM "+consecrationCatalog+" WHERE service_code=?", code); err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, ErrOrderBlessingUnavailable
		}
		return nil, err
	}
	if service.SourceServiceCode != "S006" || service.Price < 0 || math.IsNaN(service.Price) || math.IsInf(service.Price, 0) {
		return nil, ErrOrderBlessingUnavailable
	}
	service.Price = roundMoney(service.Price)
	return &service, nil
}

func orderConsecration(order *DiyOrder) (*Consecration, error) {
	var snapshot pricingSnapshot
	if err := json.Unmarshal([]byte(order.PricingSnapshot), &snapshot); err != nil {
		return nil, ErrOrderBlessingUnavailable
	}
	service := snapshot.Consecration
	if service == nil || !ValidConsecrationCode(service.ServiceCode) || service.SourceServiceCode != "S006" || service.SourceServiceId <= 0 || service.ServiceCode != fmt.Sprintf("consecration:%s:%d", service.ProviderType, service.SourceServiceId) || service.Price < 0 || math.IsNaN(service.Price) || math.IsInf(service.Price, 0) || math.Abs(service.Price-order.BlessFee) >= .005 {
		return nil, ErrOrderBlessingUnavailable
	}
	if (service.ProviderType == "temple" && service.TempleCode != "" && service.MasterCode == "") ||
		(service.ProviderType == "master" && service.MasterCode != "" && service.TempleCode == "") {
		return service, nil
	}
	return nil, ErrOrderBlessingUnavailable
}

package logic

import (
	"context"
	"errors"
	"strings"

	"github.com/askxuan/common"
	"github.com/askxuan/temple-service/internal/model"
	"github.com/askxuan/temple-service/internal/svc"
	"github.com/askxuan/temple-service/internal/types"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

var standardServiceCodes = map[string]struct{}{
	"S001": {}, "S002": {}, "S003": {}, "S004": {}, "S005": {}, "S006": {}, "S007": {},
	"S008": {}, "S009": {}, "S010": {}, "S011": {}, "S012": {}, "S013": {},
}

func ListServiceTypes(ctx context.Context, svcCtx *svc.ServiceContext) (*types.ServiceTypeListResp, error) {
	rows, err := svcCtx.ServiceTypeModel.FindAll(ctx)
	if err != nil {
		return nil, common.ErrSystem
	}
	list := make([]types.ServiceType, 0, len(rows))
	for _, row := range rows {
		if !isStandardServiceCode(row.Code) {
			continue
		}
		list = append(list, toTypeServiceType(row))
	}
	return &types.ServiceTypeListResp{List: list}, nil
}

func findServiceType(ctx context.Context, svcCtx *svc.ServiceContext, code string) (*model.ServiceType, error) {
	code = strings.TrimSpace(code)
	if !isStandardServiceCode(code) {
		return nil, common.NewBizError(40004, "请选择有效的标准服务类型")
	}
	row, err := svcCtx.ServiceTypeModel.FindOne(ctx, code)
	if err == nil {
		return row, nil
	}
	if errors.Is(err, sqlx.ErrNotFound) {
		return nil, common.NewBizError(40004, "请选择有效的标准服务类型")
	}
	return nil, common.ErrSystem
}

func isStandardServiceCode(code string) bool {
	_, ok := standardServiceCodes[strings.TrimSpace(code)]
	return ok
}

func toTypeServiceType(row *model.ServiceType) types.ServiceType {
	return types.ServiceType{
		Code: row.Code, Name: row.Name, Category: row.Category, PriceRange: row.PriceRange,
	}
}

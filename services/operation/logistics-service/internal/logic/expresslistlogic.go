package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/logistics-service/internal/model"
	"github.com/askxuan/logistics-service/internal/svc"
	"github.com/askxuan/logistics-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// ExpressListLogic 快递公司列表逻辑
type ExpressListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewExpressListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExpressListLogic {
	return &ExpressListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ExpressList 快递公司列表，支持按 code/name/status 筛选 + 分页
func (l *ExpressListLogic) ExpressList(req *types.ExpressListReq) (*types.ExpressListResp, error) {
	list, total, err := l.svcCtx.ExpressCompanyModel.FindList(l.ctx, req.Code, req.Name, req.Status, req.Page, req.Size)
	if err != nil {
		l.Errorf("查询快递公司列表失败: %v", err)
		return nil, common.ErrSystem
	}
	result := make([]types.ExpressCompany, 0, len(list))
	for _, e := range list {
		result = append(result, toTypesExpress(e))
	}
	return &types.ExpressListResp{Total: total, List: result, Page: req.Page, Size: req.Size}, nil
}

func toTypesExpress(e *model.ExpressCompany) types.ExpressCompany {
	return types.ExpressCompany{
		Id:              e.Id,
		Code:            e.Code,
		Name:            e.Name,
		LogoUrl:         e.LogoUrl,
		CustomerService: e.CustomerService,
		Sort:            e.Sort,
		Status:          e.Status,
		CreateTime:      e.CreateTime,
		UpdateTime:      e.UpdateTime,
	}
}

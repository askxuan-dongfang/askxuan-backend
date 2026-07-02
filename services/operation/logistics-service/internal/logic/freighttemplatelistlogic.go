package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/logistics-service/internal/model"
	"github.com/askxuan/logistics-service/internal/svc"
	"github.com/askxuan/logistics-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// FreightTemplateListLogic 运费模板列表逻辑
type FreightTemplateListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFreightTemplateListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FreightTemplateListLogic {
	return &FreightTemplateListLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// FreightTemplateList 运费模板列表，支持按 name/type/status 筛选 + 分页
func (l *FreightTemplateListLogic) FreightTemplateList(req *types.FreightTemplateListReq) (*types.FreightTemplateListResp, error) {
	list, total, err := l.svcCtx.FreightTemplateModel.FindList(l.ctx, req.Name, req.Type, req.Status, req.Page, req.Size)
	if err != nil {
		l.Errorf("查询运费模板列表失败: %v", err)
		return nil, common.ErrSystem
	}
	result := make([]types.FreightTemplate, 0, len(list))
	for _, t := range list {
		result = append(result, toTypesTemplate(t))
	}
	return &types.FreightTemplateListResp{Total: total, List: result, Page: req.Page, Size: req.Size}, nil
}

func toTypesTemplate(t *model.FreightTemplate) types.FreightTemplate {
	return types.FreightTemplate{
		Id:           t.Id,
		Name:         t.Name,
		Type:         t.Type,
		FreeShipping: t.FreeShipping,
		Config:       t.Config,
		Status:       t.Status,
		CreateTime:   t.CreateTime,
		UpdateTime:   t.UpdateTime,
	}
}

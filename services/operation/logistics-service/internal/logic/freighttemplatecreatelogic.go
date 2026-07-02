package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/logistics-service/internal/model"
	"github.com/askxuan/logistics-service/internal/svc"
	"github.com/askxuan/logistics-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// FreightTemplateCreateLogic 新增运费模板逻辑
type FreightTemplateCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFreightTemplateCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FreightTemplateCreateLogic {
	return &FreightTemplateCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// FreightTemplateCreate 新增运费模板（校验 type 合法性 + config JSON 格式）
func (l *FreightTemplateCreateLogic) FreightTemplateCreate(req *types.FreightTemplateCreateReq) (*types.FreightTemplateCreateResp, error) {
	if req.Type != model.FreightTypeByWeight && req.Type != model.FreightTypeByPiece {
		return nil, common.ErrParam
	}
	t, err := l.svcCtx.FreightTemplateModel.Insert(l.ctx, &model.FreightTemplate{
		Name:         req.Name,
		Type:         req.Type,
		FreeShipping: req.FreeShipping,
		Config:       req.Config,
	})
	if err != nil {
		l.Errorf("创建运费模板失败: %v", err)
		return nil, common.ErrSystem
	}
	return &types.FreightTemplateCreateResp{Id: t.Id}, nil
}

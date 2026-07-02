package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/logistics-service/internal/model"
	"github.com/askxuan/logistics-service/internal/svc"
	"github.com/askxuan/logistics-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// FreightTemplateUpdateLogic 更新运费模板逻辑
type FreightTemplateUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewFreightTemplateUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *FreightTemplateUpdateLogic {
	return &FreightTemplateUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// FreightTemplateUpdate 更新运费模板（含启停 status: enabled/disabled）
func (l *FreightTemplateUpdateLogic) FreightTemplateUpdate(req *types.FreightTemplateUpdateReq) error {
	err := l.svcCtx.FreightTemplateModel.Update(l.ctx, &model.FreightTemplate{
		Id:           req.Id,
		Name:         req.Name,
		Type:         req.Type,
		FreeShipping: req.FreeShipping,
		Config:       req.Config,
		Status:       req.Status,
	})
	if err != nil {
		l.Errorf("更新运费模板失败: %v", err)
		return common.ErrSystem
	}
	return nil
}

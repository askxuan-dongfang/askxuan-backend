package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/logistics-service/internal/model"
	"github.com/askxuan/logistics-service/internal/svc"
	"github.com/askxuan/logistics-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// ExpressUpdateLogic 更新快递公司逻辑
type ExpressUpdateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewExpressUpdateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExpressUpdateLogic {
	return &ExpressUpdateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ExpressUpdate 更新快递公司（含启停 status: enabled/disabled）
func (l *ExpressUpdateLogic) ExpressUpdate(req *types.ExpressUpdateReq) error {
	err := l.svcCtx.ExpressCompanyModel.Update(l.ctx, &model.ExpressCompany{
		Id:              req.Id,
		Name:            req.Name,
		LogoUrl:         req.LogoUrl,
		CustomerService: req.CustomerService,
		Sort:            req.Sort,
		Status:          req.Status,
	})
	if err != nil {
		l.Errorf("更新快递公司失败: %v", err)
		return common.ErrSystem
	}
	return nil
}

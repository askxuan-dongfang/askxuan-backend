package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/logistics-service/internal/model"
	"github.com/askxuan/logistics-service/internal/svc"
	"github.com/askxuan/logistics-service/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// ExpressCreateLogic 新增快递公司逻辑
type ExpressCreateLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewExpressCreateLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExpressCreateLogic {
	return &ExpressCreateLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ExpressCreate 新增快递公司（code 不可重复）
func (l *ExpressCreateLogic) ExpressCreate(req *types.ExpressCreateReq) (*types.ExpressCreateResp, error) {
	// 校验 code 唯一
	existing, _, err := l.svcCtx.ExpressCompanyModel.FindList(l.ctx, req.Code, "", "", 1, 1)
	if err != nil {
		l.Errorf("查询快递公司失败: %v", err)
		return nil, common.ErrSystem
	}
	if len(existing) > 0 {
		return nil, common.ErrDuplicateOperation
	}
	e, err := l.svcCtx.ExpressCompanyModel.Insert(l.ctx, &model.ExpressCompany{
		Code:            req.Code,
		Name:            req.Name,
		LogoUrl:         req.LogoUrl,
		CustomerService: req.CustomerService,
		Sort:            req.Sort,
	})
	if err != nil {
		l.Errorf("创建快递公司失败: %v", err)
		return nil, common.ErrSystem
	}
	return &types.ExpressCreateResp{Id: e.Id}, nil
}

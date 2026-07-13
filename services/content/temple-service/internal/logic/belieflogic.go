package logic

import (
	"context"
	"errors"

	"github.com/askxuan/common"
	"github.com/askxuan/temple-service/internal/model"
	"github.com/askxuan/temple-service/internal/svc"
	"github.com/askxuan/temple-service/internal/types"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func GetBelief(ctx context.Context, svcCtx *svc.ServiceContext, code string) (*types.BeliefProfile, error) {
	if !model.IsValidBeliefCode(code) {
		return nil, common.ErrParamInvalid
	}
	p, err := svcCtx.BeliefModel.FindOne(ctx, code)
	if err != nil {
		if errors.Is(err, sqlx.ErrNotFound) {
			return nil, common.NewBizError(40410, "信仰流派不存在")
		}
		return nil, common.ErrSystem
	}
	return &types.BeliefProfile{Code: p.Code, Name: p.Name, Summary: p.Summary, Description: p.Description, CoverImage: p.CoverImage, Sort: p.Sort}, nil
}

func UpdateBelief(ctx context.Context, svcCtx *svc.ServiceContext, req *types.BeliefUpdateReq) (*types.BeliefProfile, error) {
	if !model.IsValidBeliefCode(req.Code) || req.Name == "" || req.Description == "" {
		return nil, common.ErrParamInvalid
	}
	p := &model.BeliefProfile{Code: req.Code, Name: req.Name, Summary: req.Summary, Description: req.Description, CoverImage: req.CoverImage, Sort: req.Sort, Status: "enabled"}
	if err := svcCtx.BeliefModel.Update(ctx, p); err != nil {
		return nil, common.ErrSystem
	}
	return &types.BeliefProfile{Code: p.Code, Name: p.Name, Summary: p.Summary, Description: p.Description, CoverImage: p.CoverImage, Sort: p.Sort}, nil
}

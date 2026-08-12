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
	return beliefToType(p), nil
}

func ListBeliefs(ctx context.Context, svcCtx *svc.ServiceContext, includeDisabled bool) (*types.BeliefListResp, error) {
	profiles, err := svcCtx.BeliefModel.FindAll(ctx, includeDisabled)
	if err != nil {
		return nil, common.ErrSystem
	}
	list := make([]types.BeliefProfile, 0, len(profiles))
	for _, profile := range profiles {
		list = append(list, *beliefToType(profile))
	}
	return &types.BeliefListResp{List: list}, nil
}

func CreateBelief(ctx context.Context, svcCtx *svc.ServiceContext, req *types.BeliefCreateReq) (*types.BeliefProfile, error) {
	if !model.IsValidBeliefCode(req.Code) || req.Name == "" || req.Description == "" {
		return nil, common.ErrParamInvalid
	}
	icon := req.Icon
	if icon == "" {
		icon = "sparkles"
	}
	p := &model.BeliefProfile{Code: req.Code, Name: req.Name, Summary: req.Summary, Description: req.Description, CoverImage: req.CoverImage, Icon: icon, Sort: req.Sort, Status: "enabled"}
	if err := svcCtx.BeliefModel.Insert(ctx, p); err != nil {
		return nil, common.ErrSystem
	}
	return beliefToType(p), nil
}

func UpdateBelief(ctx context.Context, svcCtx *svc.ServiceContext, req *types.BeliefUpdateReq) (*types.BeliefProfile, error) {
	if !model.IsValidBeliefCode(req.Code) || req.Name == "" || req.Description == "" {
		return nil, common.ErrParamInvalid
	}
	icon := req.Icon
	if icon == "" {
		icon = "sparkles"
	}
	p := &model.BeliefProfile{Code: req.Code, Name: req.Name, Summary: req.Summary, Description: req.Description, CoverImage: req.CoverImage, Icon: icon, Sort: req.Sort, Status: "enabled"}
	if err := svcCtx.BeliefModel.Update(ctx, p); err != nil {
		return nil, common.ErrSystem
	}
	profiles, err := svcCtx.BeliefModel.FindAll(ctx, true)
	if err != nil {
		return nil, common.ErrSystem
	}
	for _, profile := range profiles {
		if profile.Code == req.Code {
			return beliefToType(profile), nil
		}
	}
	return nil, common.NewBizError(40410, "信仰流派不存在")
}

func UpdateBeliefStatus(ctx context.Context, svcCtx *svc.ServiceContext, req *types.BeliefStatusReq) error {
	if !model.IsValidBeliefCode(req.Code) || (req.Status != "enabled" && req.Status != "disabled") {
		return common.ErrParamInvalid
	}
	if err := svcCtx.BeliefModel.UpdateStatus(ctx, req.Code, req.Status); err != nil {
		return common.ErrSystem
	}
	return nil
}

func beliefToType(p *model.BeliefProfile) *types.BeliefProfile {
	return &types.BeliefProfile{Code: p.Code, Name: p.Name, Summary: p.Summary, Description: p.Description, CoverImage: p.CoverImage, Icon: p.Icon, Sort: p.Sort, Status: p.Status}
}

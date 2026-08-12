package logic

import (
	"context"

	"github.com/askxuan/common"
	"github.com/askxuan/product-service/internal/model"
	"github.com/askxuan/product-service/internal/svc"
	"github.com/askxuan/product-service/internal/types"
)

func CreateIntentionTag(ctx context.Context, svcCtx *svc.ServiceContext, req *types.AdminIntentionTagCreateReq) (*types.IntentionTag, error) {
	if !model.IsValidIntentCode(req.Code) || req.Name == "" || !validLandingType(req.LandingType) {
		return nil, common.ErrParamInvalid
	}
	tag := requestToIntentTag(req.Code, req.Name, req.Description, req.Icon, req.LandingType, req.LandingValue, req.ActionTitle, req.Sort)
	if err := svcCtx.IntentionModel.InsertTag(ctx, tag); err != nil {
		return nil, common.ErrSystem
	}
	result := toIntentionTag(tag)
	return &result, nil
}

func UpdateIntentionTag(ctx context.Context, svcCtx *svc.ServiceContext, req *types.AdminIntentionTagUpdateReq) (*types.IntentionTag, error) {
	if !model.IsValidIntentCode(req.Code) || req.Name == "" || !validLandingType(req.LandingType) {
		return nil, common.ErrParamInvalid
	}
	tag := requestToIntentTag(req.Code, req.Name, req.Description, req.Icon, req.LandingType, req.LandingValue, req.ActionTitle, req.Sort)
	if err := svcCtx.IntentionModel.UpdateTag(ctx, tag); err != nil {
		return nil, common.ErrSystem
	}
	stored, err := svcCtx.IntentionModel.FindTag(ctx, req.Code, false)
	if err != nil {
		return nil, common.ErrSystem
	}
	result := toIntentionTag(stored)
	return &result, nil
}

func UpdateIntentionTagStatus(ctx context.Context, svcCtx *svc.ServiceContext, req *types.AdminIntentionTagStatusReq) error {
	if !model.IsValidIntentCode(req.Code) || (req.Status != "enabled" && req.Status != "disabled") {
		return common.ErrParamInvalid
	}
	if err := svcCtx.IntentionModel.UpdateTagStatus(ctx, req.Code, req.Status); err != nil {
		return common.ErrSystem
	}
	return nil
}

func requestToIntentTag(code, name, description, icon, landingType, landingValue, actionTitle string, sort int) *model.IntentTag {
	if icon == "" {
		icon = "sparkles"
	}
	if landingType == "" {
		landingType = "aggregate"
	}
	return &model.IntentTag{Code: code, Name: name, Description: description, Icon: icon, LandingType: landingType, LandingValue: landingValue, ActionTitle: actionTitle, Sort: sort, Status: "enabled"}
}

func validLandingType(value string) bool {
	return value == "" || value == "aggregate" || value == "service" || value == "diy"
}

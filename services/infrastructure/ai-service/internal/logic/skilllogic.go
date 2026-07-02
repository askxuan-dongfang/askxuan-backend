package logic

import (
	"context"

	"github.com/askxuan/ai-service/internal/svc"
	"github.com/askxuan/ai-service/internal/types"
	"github.com/askxuan/common"

	"github.com/zeromicro/go-zero/core/logx"
)

// SkillListLogic AI 技能列表逻辑
type SkillListLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewSkillListLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SkillListLogic {
	return &SkillListLogic{Logger: logx.WithContext(ctx), ctx: ctx, svcCtx: svcCtx}
}

// SkillList 返回 7 个 AI 问事技能列表
func (l *SkillListLogic) SkillList(req *types.SkillListReq) (*types.SkillListResp, error) {
	return nil, common.ErrNotImplemented
}

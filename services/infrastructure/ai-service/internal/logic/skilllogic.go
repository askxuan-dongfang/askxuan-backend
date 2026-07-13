package logic

import (
	"context"

	"github.com/askxuan/ai-service/internal/model"
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
	skills, err := l.svcCtx.SkillModel.List(l.ctx, req.Status)
	if err != nil {
		l.Errorf("查询AI技能失败: %v", err)
		return nil, common.ErrSystem
	}
	resp := &types.SkillListResp{List: make([]types.AISkill, 0, len(skills))}
	for _, skill := range skills {
		resp.List = append(resp.List, toTypesSkill(*skill))
	}
	return resp, nil
}

func toTypesSkill(skill model.AISkill) types.AISkill {
	return types.AISkill{
		Id:             skill.Id,
		Code:           skill.Code,
		Name:           skill.Name,
		Description:    skill.Description,
		Icon:           skill.Icon,
		PromptTemplate: skill.PromptTemplate,
		Status:         skill.Status,
		CreatedAt:      skill.CreatedAt,
	}
}

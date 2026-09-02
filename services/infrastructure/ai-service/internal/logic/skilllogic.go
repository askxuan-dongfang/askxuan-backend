package logic

import (
	"context"
	"encoding/json"

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

// SkillList 返回平台启用的动态智能体技能目录。
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
	inputSchema := json.RawMessage(skill.InputSchema)
	if !json.Valid(inputSchema) {
		inputSchema = json.RawMessage(`{"fields":[]}`)
	}
	capabilities := json.RawMessage(skill.Capabilities)
	if !json.Valid(capabilities) {
		capabilities = json.RawMessage(`[]`)
	}
	return types.AISkill{
		Id: skill.Id, Code: skill.Code, Category: skill.Category, Name: skill.Name,
		Version: skill.Version, Description: skill.Description, Icon: skill.Icon,
		SourceType: skill.SourceType, SourceRef: skill.SourceRef, InputSchema: inputSchema,
		Capabilities: capabilities, RiskLevel: skill.RiskLevel, SortOrder: skill.SortOrder,
		Status: skill.Status, CreatedAt: skill.CreatedAt,
	}
}

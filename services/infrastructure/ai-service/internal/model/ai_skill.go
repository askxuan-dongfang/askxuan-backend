package model

import (
	"sync"
)

// 技能启停
const (
	SkillStatusEnabled  = "enabled"
	SkillStatusDisabled = "disabled"
)

// 7 个技能编码
const (
	SkillCodeBazi     = "bazi"     // 八字命理
	SkillCodeMarriage = "marriage" // 姻缘测算
	SkillCodeTarot    = "tarot"    // 塔罗牌
	SkillCodeFengshui = "fengshui" // 风水分析
	SkillCodeQimen    = "qimen"    // 奇门遁甲
	SkillCodeZiwei    = "ziwei"    // 紫微斗数
	SkillCodeLiuyao   = "liuyao"   // 六爻梅花
)

// AISkill AI 问事技能
type AISkill struct {
	Id             int64  `json:"id"`
	Code           string `json:"code"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Icon           string `json:"icon"`
	PromptTemplate string `json:"promptTemplate"`
	Status         string `json:"status"`
	CreatedAt      string `json:"createdAt"`
}

// ---- 内存存储 ----

type skillStore struct {
	mu   sync.RWMutex
	list []AISkill
}

var globalSkillStore = &skillStore{
	list: []AISkill{
		{Id: 1, Code: SkillCodeBazi, Name: "八字命理", Description: "依据生辰八字推演命格运势", Icon: "/icons/bazi.png", PromptTemplate: "你是一位精通八字命理的师傅，请根据用户八字解答...", Status: SkillStatusEnabled, CreatedAt: "2026-06-28 10:00:00"},
		{Id: 2, Code: SkillCodeMarriage, Name: "姻缘测算", Description: "测算姻缘婚恋走势", Icon: "/icons/marriage.png", PromptTemplate: "你是一位姻缘测算师，请根据用户信息解答感情问题...", Status: SkillStatusEnabled, CreatedAt: "2026-06-28 10:00:00"},
		{Id: 3, Code: SkillCodeTarot, Name: "塔罗牌", Description: "塔罗牌占卜指引", Icon: "/icons/tarot.png", PromptTemplate: "你是一位塔罗牌占卜师，请为用户抽牌并解读...", Status: SkillStatusEnabled, CreatedAt: "2026-06-28 10:00:00"},
		{Id: 4, Code: SkillCodeFengshui, Name: "风水分析", Description: "居家风水布局建议", Icon: "/icons/fengshui.png", PromptTemplate: "你是一位风水师，请根据用户描述分析风水布局...", Status: SkillStatusEnabled, CreatedAt: "2026-06-28 10:00:00"},
		{Id: 5, Code: SkillCodeQimen, Name: "奇门遁甲", Description: "奇门遁甲预测决策", Icon: "/icons/qimen.png", PromptTemplate: "你是一位奇门遁甲大师，请根据用户问题起局预测...", Status: SkillStatusEnabled, CreatedAt: "2026-06-28 10:00:00"},
		{Id: 6, Code: SkillCodeZiwei, Name: "紫微斗数", Description: "紫微斗数命盘解析", Icon: "/icons/ziwei.png", PromptTemplate: "你是一位紫微斗数师傅，请根据用户命盘解析运势...", Status: SkillStatusEnabled, CreatedAt: "2026-06-28 10:00:00"},
		{Id: 7, Code: SkillCodeLiuyao, Name: "六爻梅花", Description: "六爻梅花易数占断", Icon: "/icons/liuyao.png", PromptTemplate: "你是一位六爻梅花易数师傅，请根据用户问题起卦占断...", Status: SkillStatusEnabled, CreatedAt: "2026-06-28 10:00:00"},
	},
}

// ===== Skill =====

// ListSkills 技能列表
func ListSkills(status string) []AISkill {
	globalSkillStore.mu.RLock()
	defer globalSkillStore.mu.RUnlock()
	out := make([]AISkill, 0, len(globalSkillStore.list))
	for _, s := range globalSkillStore.list {
		if status != "" && s.Status != status {
			continue
		}
		out = append(out, s)
	}
	return out
}

// FindSkillByCode 按编码查询技能
func FindSkillByCode(code string) (AISkill, bool) {
	globalSkillStore.mu.RLock()
	defer globalSkillStore.mu.RUnlock()
	for _, s := range globalSkillStore.list {
		if s.Code == code {
			return s, true
		}
	}
	return AISkill{}, false
}

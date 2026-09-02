package svc

import (
	"context"

	"github.com/askxuan/ai-service/internal/agent"
	"github.com/askxuan/ai-service/internal/config"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/provider"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ServiceContext ai 服务依赖容器
type ServiceContext struct {
	Config            config.Config
	DB                sqlx.SqlConn
	SkillModel        model.SkillModel
	ConversationModel model.ConversationModel
	UsageModel        model.UsageModel
	Provider          provider.Provider
	Guard             *agent.Guard
	MCP               *agent.MCPClient
	AIConfig          config.AIConf
}

func NewServiceContext(c config.Config) *ServiceContext {
	db := sqlx.NewMysql(c.MySQL.DataSource)
	runtimeAI := c.AI.Runtime()
	providerConfig := provider.Config{Provider: runtimeAI.Provider, BaseURL: runtimeAI.BaseURL, APIKey: runtimeAI.APIKey, Model: runtimeAI.Model}
	aiProvider, err := provider.New(providerConfig)
	if err != nil {
		panic(err)
	}
	conversationModel := model.NewConversationModel(db)
	if err := conversationModel.RecoverPending(context.Background()); err != nil {
		logx.Errorf("恢复AI待处理消息失败: %v", err)
	}
	return &ServiceContext{
		Config:            c,
		DB:                db,
		SkillModel:        model.NewSkillModel(db),
		ConversationModel: conversationModel,
		UsageModel:        model.NewUsageModel(db),
		Provider:          aiProvider,
		Guard:             agent.NewGuard(runtimeAI.MaxInputChars, runtimeAI.BlockedTerms),
		MCP:               agent.NewMCPClient(runtimeAI.MCP.Enabled, runtimeAI.MCP.BaseURL, runtimeAI.MCP.Timeout),
		AIConfig:          runtimeAI,
	}
}

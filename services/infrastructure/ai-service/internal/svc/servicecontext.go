package svc

import (
	"context"
	"os"

	"github.com/askxuan/ai-service/internal/agent"
	"github.com/askxuan/ai-service/internal/config"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/askxuan/ai-service/internal/settings"

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
	RunModel          model.RunModel
	Provider          provider.Provider
	Models            *provider.Catalog
	Guard             *agent.Guard
	MCP               *agent.MCPClient
	ImageLoader       *agent.ImageLoader
	AIConfig          config.AIConf
	Settings          *settings.Manager
}

func NewServiceContext(c config.Config) *ServiceContext {
	db := sqlx.NewMysql(c.MySQL.DataSource)
	runtimeAI := c.AI.Runtime()
	manager, err := settings.New(runtimeAI, os.Getenv("AI_SETTINGS_DIR"), os.Getenv("AI_SETTINGS_ENCRYPTION_KEY"))
	if err != nil {
		panic(err)
	}
	snapshot := manager.Snapshot()
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
		RunModel:          model.NewRunModel(db),
		Provider:          snapshot.Provider,
		Models:            snapshot.Models,
		Guard:             agent.NewGuard(runtimeAI.MaxInputChars, runtimeAI.BlockedTerms),
		MCP:               agent.NewMCPClient(runtimeAI.MCP.Enabled, runtimeAI.MCP.BaseURL, runtimeAI.MCP.Timeout),
		ImageLoader:       agent.NewImageLoader(runtimeAI.AllowedImageHosts, runtimeAI.ImageMaxBytes),
		AIConfig:          snapshot.Config,
		Settings:          manager,
	}
}

// Each request retains one immutable provider/config snapshot, including async work.
func (s *ServiceContext) Runtime() *ServiceContext {
	if s.Settings == nil {
		return s
	}
	snapshot := s.Settings.Snapshot()
	result := *s
	result.Provider = snapshot.Provider
	result.Models = snapshot.Models
	result.AIConfig = snapshot.Config
	result.Settings = nil
	return &result
}

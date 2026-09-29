package svc

import (
	"context"
	"os"

	"github.com/askxuan/ai-service/internal/agent"
	"github.com/askxuan/ai-service/internal/agentops"
	"github.com/askxuan/ai-service/internal/config"
	"github.com/askxuan/ai-service/internal/knowledge"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/provider"
	"github.com/askxuan/ai-service/internal/settings"
	"github.com/askxuan/ai-service/internal/weknora"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

// ServiceContext ai 服务依赖容器
type ServiceContext struct {
	KnowledgeBases    []string
	WeKnora           *weknora.Service
	Knowledge         *knowledge.Store
	KnowledgeEnabled  bool
	MemoryEnabled     bool
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
	AgentOps          *agentops.Manager
	AgentDefaultModel string
	AgentVersion      int64
	AgentSubject      string
}

func NewServiceContext(c config.Config) *ServiceContext {
	// SQL parameters contain private messages, naming inputs and journal notes.
	// Keep request metadata logs, but never interpolate these values into SQL logs.
	sqlx.DisableLog()
	db := sqlx.NewMysql(c.MySQL.DataSource)
	runtimeAI := c.AI.Runtime()
	manager, err := settings.New(runtimeAI, os.Getenv("AI_SETTINGS_DIR"), os.Getenv("AI_SETTINGS_ENCRYPTION_KEY"))
	if err != nil {
		panic(err)
	}
	snapshot := manager.Snapshot()
	mcp := agent.NewMCPClient(runtimeAI.MCP.Enabled, runtimeAI.MCP.BaseURL, runtimeAI.MCP.Timeout)
	skills := model.NewSkillModel(db)
	conversationModel := model.NewConversationModel(db)
	if err := conversationModel.RecoverPending(context.Background()); err != nil {
		logx.Errorf("恢复AI待处理消息失败: %v", err)
	}
	ops := agentops.New(&agentops.SQLRepository{DB: db, RuntimeMode: func() string {
		if runtimeAI.HarnessEnabled {
			return "harness"
		}
		return "classic"
	}()}, skills, manager.VersionedSnapshot, mcp, os.Getenv("AI_AGENT_OPERATIONS_ENABLED") == "true")
	ops.RuntimeMode = "classic"
	if runtimeAI.HarnessEnabled {
		ops.RuntimeMode = "harness"
	}
	if err := ops.EnablePersistence(os.Getenv("AI_SETTINGS_ENCRYPTION_KEY")); err != nil {
		panic(err)
	}
	references := &knowledge.Store{DB: db, Embedder: knowledge.EmbeddingFromEnv()}
	var wk *weknora.Service
	if client := weknora.FromEnv(); client != nil {
		wk = &weknora.Service{DB: db, Client: client}
		references.Remote = wk
	}
	ops.References = references
	return &ServiceContext{
		Knowledge:         references,
		WeKnora:           wk,
		Config:            c,
		DB:                db,
		SkillModel:        skills,
		ConversationModel: conversationModel,
		UsageModel:        model.NewUsageModel(db),
		RunModel:          model.NewRunModel(db),
		Provider:          snapshot.Provider,
		Models:            snapshot.Models,
		Guard:             agent.NewGuard(snapshot.Config.MaxInputChars, snapshot.Config.BlockedTerms),
		MCP:               mcp,
		ImageLoader:       agent.NewImageLoader(runtimeAI.AllowedImageHosts, runtimeAI.ImageMaxBytes),
		AIConfig:          snapshot.Config,
		Settings:          manager,
		AgentOps:          ops,
	}
}

// Each request retains one immutable provider/config snapshot, including async work.

// AskRuntime pins published skills before accepting a chat turn. An unavailable
// store fails the turn instead of silently reverting an active policy.
func (s *ServiceContext) AskRuntime(ctx context.Context) (*ServiceContext, error) {
	return s.AskRuntimeFor(ctx, s.AgentSubject)
}
func (s *ServiceContext) AskRuntimeFor(ctx context.Context, subject string) (*ServiceContext, error) {
	if s.AgentOps == nil {
		return s, nil
	}
	f, version, err := s.AgentOps.ActiveFor(ctx, subject)
	if err != nil {
		return nil, err
	}
	if f == nil {
		return s, nil
	}
	result := *s
	for i := range f.Skills {
		f.Skills[i].PromptTemplate = f.Config.Instruction + "\n" + f.Skills[i].PromptTemplate
	}
	result.SkillModel = &agentops.SkillView{Frozen: *f, Version: version}
	result.AgentDefaultModel = f.Config.Model
	result.AgentVersion = version
	result.KnowledgeEnabled = f.Config.KnowledgeEnabled
	result.KnowledgeBases = append([]string(nil), f.Config.KnowledgeBaseIDs...)
	result.MemoryEnabled = f.Config.MemoryEnabled
	if result.AIConfig.ComplexOutputTokens <= 0 || f.Config.MaxOutputTokens < result.AIConfig.ComplexOutputTokens {
		result.AIConfig.ComplexOutputTokens = f.Config.MaxOutputTokens
	}
	if result.AIConfig.MaxOutputTokens <= 0 || f.Config.MaxOutputTokens < result.AIConfig.MaxOutputTokens {
		result.AIConfig.MaxOutputTokens = f.Config.MaxOutputTokens
	}
	return &result, nil
}

func (s *ServiceContext) Runtime() *ServiceContext {
	if s.Settings == nil {
		return s
	}
	snapshot := s.Settings.Snapshot()
	result := *s
	result.Provider = snapshot.Provider
	result.Models = snapshot.Models
	result.AIConfig = snapshot.Config
	result.Guard = agent.NewGuard(snapshot.Config.MaxInputChars, snapshot.Config.BlockedTerms)
	result.Settings = nil
	return &result
}

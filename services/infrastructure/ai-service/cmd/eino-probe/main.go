// eino-probe runs one bounded, read-only task using an existing AI configuration.
// It neither registers product routes nor writes settings, orders, usage or money.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/askxuan/ai-service/internal/agent"
	"github.com/askxuan/ai-service/internal/config"
	"github.com/askxuan/ai-service/internal/einopoc"
	"github.com/askxuan/ai-service/internal/model"
	"github.com/askxuan/ai-service/internal/settings"
	"github.com/cloudwego/eino/components/tool"
	"github.com/zeromicro/go-zero/core/conf"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
)

func main() {
	configFile := flag.String("config", "", "existing ai.yaml; credentials remain in the existing settings store")
	skillCode := flag.String("skill", "bazi", "read-only skill: bazi, ziwei or qimen")
	modelID := flag.String("model", "", "enabled model ID; blank uses existing default")
	inputsFile := flag.String("inputs", "", "JSON file containing synthetic, user-confirmed input facts")
	replyFile := flag.String("reply", "", "optional JSON clarification to resume in this process")
	flag.Parse()
	fail := func(stage string) {
		_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"ok": false, "stage": stage})
		os.Exit(1)
	}
	if *configFile == "" {
		fmt.Fprintln(os.Stderr, "-config is required; this probe sends a small number of billable model requests")
		os.Exit(2)
	}
	var c config.Config
	if conf.Load(*configFile, &c) != nil {
		fail("configuration")
	}
	runtime := c.AI.Runtime()
	manager, err := settings.New(runtime, os.Getenv("AI_SETTINGS_DIR"), os.Getenv("AI_SETTINGS_ENCRYPTION_KEY"))
	if err != nil {
		fail("settings")
	}
	// Reading a snapshot does not invoke Save or update the encrypted store.
	snapshot := manager.Snapshot()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	sqlx.DisableLog()
	db := sqlx.NewMysql(c.MySQL.DataSource)
	skill, err := model.NewSkillModel(db).FindByCode(ctx, *skillCode)
	if err != nil {
		fail("skill_lookup")
	}
	inputs := map[string]any{}
	read := func(path string) []byte {
		b, e := os.ReadFile(path)
		if e != nil || len(b) > 8000 {
			fail("input_file")
		}
		return b
	}
	if *inputsFile != "" {
		if json.Unmarshal(read(*inputsFile), &inputs) != nil {
			fail("input_json")
		}
	}
	question := "这是合成资料的接入验证，请调用对应计算工具，只简短说明工具是否成功和结果类型，不作预测，不调用无关能力。"
	guard := agent.NewGuard(snapshot.Config.MaxInputChars, snapshot.Config.BlockedTerms)
	bound, err := einopoc.NewSkillTool(*skill, inputs, question, agent.NewMCPClient(runtime.MCP.Enabled, runtime.MCP.BaseURL, runtime.MCP.Timeout), guard)
	if err != nil {
		fail("skill_binding")
	}
	h, err := einopoc.FromSnapshot(ctx, snapshot, *modelID, []tool.BaseTool{bound}, einopoc.Limits{ModelCalls: 4, ToolCalls: 4, Timeout: 45 * time.Second})
	if err != nil {
		fail("model_setup")
	}
	result, err := h.Run(ctx, question, nil)
	if err != nil {
		fail("agent_run")
	}
	resumed := false
	if result.InterruptID != "" && *replyFile != "" {
		result, err = h.Resume(ctx, result.InterruptID, string(read(*replyFile)), nil)
		resumed = true
		if err != nil {
			fail("agent_resume")
		}
	}
	models, tools := h.Counts()
	selected := *modelID
	if selected == "" {
		selected = snapshot.Provider.Model()
	}
	_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"ok": true, "model": selected, "skill": skill.Code, "resumed": resumed, "modelAttempts": models, "toolAttempts": tools, "result": result})
}

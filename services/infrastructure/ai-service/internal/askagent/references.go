package askagent

import (
	"context"
	"encoding/json"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"strings"
)

type referenceTool struct {
	name, desc string
	read       func(context.Context, string, string) (string, error)
	hooks      Hooks
}

func (t *referenceTool) Info(context.Context) (*schema.ToolInfo, error) {
	params := map[string]*schema.ParameterInfo{}
	desc := t.desc + " 此工具无参数，只能传空对象 {}；查询和用户身份由平台绑定。"
	if t.name == "search_knowledge" {
		params["query"] = &schema.ParameterInfo{Type: schema.String, Desc: "可选检索词，1至400字；提炼书名、章节和关键概念。省略则使用当前问题。"}
		desc = t.desc + " 可传 query 提炼检索词；用户身份及知识库范围始终由平台绑定，无法修改。"
	}
	return &schema.ToolInfo{Name: t.name, Desc: desc, ParamsOneOf: schema.NewParamsOneOfByParams(params)}, nil
}

func (t *referenceTool) InvokableRun(ctx context.Context, args string, _ ...tool.Option) (string, error) {
	var a map[string]any
	valid := json.Unmarshal([]byte(args), &a) == nil && a != nil
	query := ""
	if valid && len(a) > 0 {
		value, ok := a["query"].(string)
		query = strings.TrimSpace(value)
		valid = t.name == "search_knowledge" && len(a) == 1 && ok && query != "" && len([]rune(query)) <= 400
	}
	if !valid {
		return `{"ok":false,"error":"invalid_model_arguments","message":"检索未执行。search_knowledge 仅接受空对象或 query 字符串（1至400字）；recall_memory 仅接受空对象。身份与已授权知识库由平台绑定，不能修改。不能依据被拒绝的参数生成引用或记忆。"}`, nil
	}
	normalized := "{}"
	if query != "" {
		b, _ := json.Marshal(map[string]string{"query": query})
		normalized = string(b)
	}
	fn := func() (string, error) { return t.read(ctx, t.name, query) }
	if t.hooks.Call != nil {
		return t.hooks.Call(ctx, "local", t.name, normalized, fn)
	}
	return fn()
}

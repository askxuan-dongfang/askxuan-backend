package askagent

import (
	"context"
	"encoding/json"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

type referenceTool struct {
	name, desc string
	read       func(context.Context, string) (string, error)
	hooks      Hooks
}

func (t *referenceTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: t.name, Desc: t.desc + " 此工具无参数，只能传空对象 {}；查询、用户身份和知识库范围由平台绑定。", ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{})}, nil
}
func (t *referenceTool) InvokableRun(ctx context.Context, args string, _ ...tool.Option) (string, error) {
	var a map[string]any
	if json.Unmarshal([]byte(args), &a) != nil || a == nil || len(a) != 0 {
		return `{"ok":false,"error":"invalid_model_arguments","message":"检索未执行。此工具无参数，请仅用空对象 {} 重试；平台自动绑定当前问题、用户身份与已授权知识库。不能依据被拒绝的参数生成引用或记忆。"}`, nil
	}
	fn := func() (string, error) { return t.read(ctx, t.name) }
	if t.hooks.Call != nil {
		return t.hooks.Call(ctx, "local", t.name, "{}", fn)
	}
	return fn()
}

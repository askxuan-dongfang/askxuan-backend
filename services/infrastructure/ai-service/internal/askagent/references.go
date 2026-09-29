package askagent

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
)

type referenceTool struct {
	name, desc string
	read       func(context.Context, string) (string, error)
	hooks      Hooks
}

func (t *referenceTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: t.name, Desc: t.desc, ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{})}, nil
}
func (t *referenceTool) InvokableRun(ctx context.Context, args string, _ ...tool.Option) (string, error) {
	var a map[string]any
	if json.Unmarshal([]byte(args), &a) != nil || a == nil || len(a) != 0 {
		return "", errors.New("reference tool accepts no model-supplied identity or arguments")
	}
	fn := func() (string, error) { return t.read(ctx, t.name) }
	if t.hooks.Call != nil {
		return t.hooks.Call(ctx, "local", t.name, "{}", fn)
	}
	return fn()
}

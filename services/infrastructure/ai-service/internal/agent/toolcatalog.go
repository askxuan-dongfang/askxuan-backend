package agent

import (
	_ "embed"
	"encoding/json"
)

// Reviewed tool identities and presentation metadata, pinned to the deployed MCP contract.
// Remote discovery never automatically grants a model access to a new tool.
//
//go:embed tool_catalog.json
var toolCatalogJSON []byte

type ToolDefinition struct {
	Code        string      `json:"code"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
	Keywords    []string    `json:"keywords"`
	Group       string      `json:"group"`
}

func ReviewedTools() []ToolDefinition {
	var list []ToolDefinition
	if err := json.Unmarshal(toolCatalogJSON, &list); err != nil {
		panic("invalid reviewed tool catalog")
	}
	return list
}
func IsReadOnlyTool(code string) bool {
	for _, t := range ReviewedTools() {
		if t.Code == code {
			return true
		}
	}
	return false
}

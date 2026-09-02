package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type ToolConfig struct {
	Enabled bool   `json:"enabled"`
	Server  string `json:"server"`
	Tool    string `json:"tool"`
}

type MCPClient struct {
	enabled bool
	baseURL string
	client  *http.Client
}

func NewMCPClient(enabled bool, baseURL string, timeoutSeconds int) *MCPClient {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 10
	}
	return &MCPClient{
		enabled: enabled,
		baseURL: strings.TrimSpace(baseURL),
		client:  &http.Client{Timeout: time.Duration(timeoutSeconds) * time.Second},
	}
}

// Call executes only a server-controlled, per-skill allowlisted tool.
func (c *MCPClient) Call(ctx context.Context, configJSON, argumentsJSON string) (string, error) {
	var config ToolConfig
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return "", fmt.Errorf("decode MCP tool config: %w", err)
	}
	if !c.enabled || !config.Enabled {
		return "", nil
	}
	if c.baseURL == "" || config.Tool == "" {
		return "", fmt.Errorf("MCP enabled without base URL or tool")
	}
	arguments := map[string]interface{}{}
	if strings.TrimSpace(argumentsJSON) != "" {
		if err := json.Unmarshal([]byte(argumentsJSON), &arguments); err != nil {
			return "", fmt.Errorf("decode MCP arguments: %w", err)
		}
	}
	payload := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      fmt.Sprintf("askxuan-%d", time.Now().UnixNano()),
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name": config.Tool, "arguments": arguments,
			"_meta": map[string]string{"client": "askxuan-ai-service"},
		},
	}
	body, _ := json.Marshal(payload)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json, text/event-stream")
	request.Header.Set("MCP-Protocol-Version", "2025-03-26")
	request.Header.Set("Mcp-Method", "tools/call")
	request.Header.Set("Mcp-Name", config.Tool)
	response, err := c.client.Do(request)
	if err != nil {
		return "", fmt.Errorf("MCP request failed: %w", err)
	}
	defer response.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(response.Body, 4<<20))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", fmt.Errorf("MCP returned %d: %s", response.StatusCode, strings.TrimSpace(string(data)))
	}
	if strings.HasPrefix(strings.TrimSpace(string(data)), "data:") {
		data = firstSSEData(data)
	}
	var decoded struct {
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
		Result struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
			StructuredContent interface{} `json:"structuredContent"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return "", fmt.Errorf("decode MCP response: %w", err)
	}
	if decoded.Error != nil {
		return "", fmt.Errorf("MCP tool failed: %s", decoded.Error.Message)
	}
	parts := make([]string, 0, len(decoded.Result.Content)+1)
	for _, item := range decoded.Result.Content {
		if text := strings.TrimSpace(item.Text); text != "" {
			parts = append(parts, text)
		}
	}
	if decoded.Result.StructuredContent != nil {
		structured, _ := json.Marshal(decoded.Result.StructuredContent)
		parts = append(parts, string(structured))
	}
	return strings.Join(parts, "\n"), nil
}

func firstSSEData(data []byte) []byte {
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "data:") {
			return []byte(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "data:")))
		}
	}
	return data
}

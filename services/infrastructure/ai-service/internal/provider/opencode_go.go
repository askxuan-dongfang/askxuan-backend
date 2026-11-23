package provider

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"sort"
	"strings"
	"time"
)

const (
	OpenCodeGo        = "opencode_go"
	OpenCodeGoBaseURL = "https://opencode.ai/inference/go/openai/v1"
	openCodeGoModels  = "https://opencode.ai/inference/go/v1/models"
	openCodeUserAgent = "AskXuan/1.0"
)

func DiscoverOpenCodeGo(ctx context.Context, client *http.Client, key string) ([]ModelOption, error) {
	ctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, openCodeGoModels, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("User-Agent", openCodeUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New("OpenCode Go 模型列表暂时无法连接")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &HTTPError{StatusCode: resp.StatusCode}
	}
	var reply struct {
		Object string          `json:"object"`
		Error  json.RawMessage `json:"error"`
		Data   []struct {
			Type  string `json:"type"`
			ID    string `json:"id"`
			Name  string `json:"name"`
			Limit struct {
				Context int `json:"context"`
				Output  int `json:"output"`
			} `json:"limit"`
			Modalities struct {
				Input  []string `json:"input"`
				Output []string `json:"output"`
			} `json:"modalities"`
			API map[string]struct {
				BaseURL string `json:"base_url"`
			} `json:"api"`
		} `json:"data"`
	}
	const maxCatalog = 2 << 20
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxCatalog+1))
	if err != nil || len(raw) > maxCatalog || json.Unmarshal(raw, &reply) != nil ||
		reply.Object != "list" || (len(reply.Error) > 0 && string(reply.Error) != "null") || reply.Data == nil {
		return nil, errors.New("OpenCode Go 模型列表响应无效")
	}
	options := make([]ModelOption, 0, len(reply.Data))
	seen := make(map[string]bool)
	for _, entry := range reply.Data {
		if entry.Type != "chat" || !modelID.MatchString(entry.ID) || seen[entry.ID] ||
			strings.TrimSuffix(entry.API["chat-completions"].BaseURL, "/") != OpenCodeGoBaseURL ||
			(len(entry.Modalities.Input) > 0 && !slices.Contains(entry.Modalities.Input, "text")) ||
			(len(entry.Modalities.Output) > 0 && !slices.Contains(entry.Modalities.Output, "text")) {
			continue
		}
		seen[entry.ID] = true
		name := strings.TrimSpace(entry.Name)
		if name == "" || len(name) > 120 {
			name = entry.ID
		}
		options = append(options, ModelOption{
			ID: entry.ID, Name: name, ContextWindow: max(0, entry.Limit.Context), MaxOutputTokens: max(0, entry.Limit.Output),
			SupportsVision: slices.ContainsFunc(entry.Modalities.Input, func(m string) bool {
				return m == "image" || strings.HasPrefix(m, "image/")
			}),
		})
	}
	sort.Slice(options, func(i, j int) bool { return options[i].ID < options[j].ID })
	return options, nil
}

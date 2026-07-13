package provider

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

type OpenAICompatible struct {
	baseURL, apiKey, model string
	client                 *http.Client
}

func NewOpenAICompatible(baseURL, apiKey, model string) *OpenAICompatible {
	return &OpenAICompatible{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, model: model, client: &http.Client{Timeout: 60 * time.Second}}
}
func (p *OpenAICompatible) Name() string { return "openai_compatible" }
func (p *OpenAICompatible) Complete(ctx context.Context, req Request) (*Response, error) {
	messages := make([]Message, 0, len(req.Messages)+1)
	if req.SystemPrompt != "" {
		messages = append(messages, Message{Role: "system", Content: req.SystemPrompt})
	}
	messages = append(messages, req.Messages...)
	body, _ := json.Marshal(map[string]interface{}{"model": p.model, "messages": messages})
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("provider request failed: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("provider returned %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	var decoded struct {
		Choices []struct {
			Message Message `json:"message"`
		} `json:"choices"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("decode provider response: %w", err)
	}
	if len(decoded.Choices) == 0 || strings.TrimSpace(decoded.Choices[0].Message.Content) == "" {
		return nil, fmt.Errorf("provider returned empty content")
	}
	return &Response{Content: decoded.Choices[0].Message.Content, Tokens: decoded.Usage.TotalTokens}, nil
}

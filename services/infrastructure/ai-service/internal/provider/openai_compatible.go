package provider

import (
	"bufio"
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
func (p *OpenAICompatible) Name() string  { return "openai_compatible" }
func (p *OpenAICompatible) Model() string { return p.model }
func (p *OpenAICompatible) Complete(ctx context.Context, req Request) (*Response, error) {
	messages := make([]Message, 0, len(req.Messages)+1)
	if req.SystemPrompt != "" {
		messages = append(messages, Message{Role: "system", Content: req.SystemPrompt})
	}
	messages = append(messages, req.Messages...)
	payload := map[string]interface{}{"model": p.model, "messages": messages}
	if req.MaxTokens > 0 {
		payload["max_tokens"] = req.MaxTokens
	}
	body, _ := json.Marshal(payload)
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
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("decode provider response: %w", err)
	}
	if len(decoded.Choices) == 0 || strings.TrimSpace(decoded.Choices[0].Message.Content) == "" {
		return nil, fmt.Errorf("provider returned empty content")
	}
	return &Response{Content: decoded.Choices[0].Message.Content, PromptTokens: decoded.Usage.PromptTokens, CompletionTokens: decoded.Usage.CompletionTokens, FinishReason: "stop"}, nil
}

func (p *OpenAICompatible) Stream(ctx context.Context, req Request, onDelta func(string) error) (*Response, error) {
	messages := make([]Message, 0, len(req.Messages)+1)
	if req.SystemPrompt != "" {
		messages = append(messages, Message{Role: "system", Content: req.SystemPrompt})
	}
	messages = append(messages, req.Messages...)
	payload := map[string]interface{}{
		"model":          p.model,
		"messages":       messages,
		"stream":         true,
		"stream_options": map[string]bool{"include_usage": true},
	}
	if req.MaxTokens > 0 {
		payload["max_tokens"] = req.MaxTokens
	}
	body, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("provider stream request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
		return nil, fmt.Errorf("provider returned %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	result := &Response{}
	var content strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 64*1024), 2<<20)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" || data == "[DONE]" {
			continue
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Usage struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return nil, fmt.Errorf("decode provider stream: %w", err)
		}
		result.PromptTokens = chunk.Usage.PromptTokens
		result.CompletionTokens = chunk.Usage.CompletionTokens
		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				content.WriteString(choice.Delta.Content)
				if err := onDelta(choice.Delta.Content); err != nil {
					return nil, err
				}
			}
			if choice.FinishReason != "" {
				result.FinishReason = choice.FinishReason
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read provider stream: %w", err)
	}
	result.Content = content.String()
	if strings.TrimSpace(result.Content) == "" {
		return nil, fmt.Errorf("provider returned empty streamed content")
	}
	if result.FinishReason == "" {
		result.FinishReason = "stop"
	}
	return result, nil
}

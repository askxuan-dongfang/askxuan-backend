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
	baseURL, apiKey, model, visionModel string
	client                              *http.Client
}

func NewOpenAICompatible(baseURL, apiKey, model, visionModel string) *OpenAICompatible {
	return &OpenAICompatible{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, model: model, visionModel: visionModel, client: &http.Client{Timeout: 60 * time.Second}}
}
func (p *OpenAICompatible) Name() string  { return "openai_compatible" }
func (p *OpenAICompatible) Model() string { return p.model }
func (p *OpenAICompatible) ModelFor(req Request) string {
	if p.visionModel != "" {
		for _, message := range req.Messages {
			if len(message.ImageDataURLs) > 0 {
				return p.visionModel
			}
		}
	}
	return p.model
}
func (p *OpenAICompatible) Complete(ctx context.Context, req Request) (*Response, error) {
	messages := wireMessages(req)
	model := p.ModelFor(req)
	payload := basePayload(req, model, messages)
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
			PromptTokens           int `json:"prompt_tokens"`
			CompletionTokens       int `json:"completion_tokens"`
			PromptCacheHitTokens   int `json:"prompt_cache_hit_tokens"`
			PromptCacheMissTokens  int `json:"prompt_cache_miss_tokens"`
			CompletionTokenDetails struct {
				ReasoningTokens int `json:"reasoning_tokens"`
			} `json:"completion_tokens_details"`
		} `json:"usage"`
		Model string `json:"model"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, fmt.Errorf("decode provider response: %w", err)
	}
	if len(decoded.Choices) == 0 || strings.TrimSpace(decoded.Choices[0].Message.Content) == "" {
		return nil, fmt.Errorf("provider returned empty content")
	}
	return &Response{Content: decoded.Choices[0].Message.Content, PromptTokens: decoded.Usage.PromptTokens, CompletionTokens: decoded.Usage.CompletionTokens, PromptCacheHitTokens: decoded.Usage.PromptCacheHitTokens, PromptCacheMissTokens: decoded.Usage.PromptCacheMissTokens, ReasoningTokens: decoded.Usage.CompletionTokenDetails.ReasoningTokens, FinishReason: "stop", Model: defaultValue(decoded.Model, model)}, nil
}

func (p *OpenAICompatible) Stream(ctx context.Context, req Request, onDelta func(StreamDelta) error) (*Response, error) {
	messages := wireMessages(req)
	model := p.ModelFor(req)
	payload := basePayload(req, model, messages)
	payload["stream"] = true
	payload["stream_options"] = map[string]bool{"include_usage": true}
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
					Content          string `json:"content"`
					ReasoningContent string `json:"reasoning_content"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
			Usage struct {
				PromptTokens           int `json:"prompt_tokens"`
				CompletionTokens       int `json:"completion_tokens"`
				PromptCacheHitTokens   int `json:"prompt_cache_hit_tokens"`
				PromptCacheMissTokens  int `json:"prompt_cache_miss_tokens"`
				CompletionTokenDetails struct {
					ReasoningTokens int `json:"reasoning_tokens"`
				} `json:"completion_tokens_details"`
			} `json:"usage"`
			Model string `json:"model"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return nil, fmt.Errorf("decode provider stream: %w", err)
		}
		result.PromptTokens = chunk.Usage.PromptTokens
		result.CompletionTokens = chunk.Usage.CompletionTokens
		result.PromptCacheHitTokens = chunk.Usage.PromptCacheHitTokens
		result.PromptCacheMissTokens = chunk.Usage.PromptCacheMissTokens
		result.ReasoningTokens = chunk.Usage.CompletionTokenDetails.ReasoningTokens
		if chunk.Model != "" {
			result.Model = chunk.Model
		}
		for _, choice := range chunk.Choices {
			if choice.Delta.ReasoningContent != "" {
				if err := onDelta(StreamDelta{Reasoning: true}); err != nil {
					return nil, err
				}
			}
			if choice.Delta.Content != "" {
				content.WriteString(choice.Delta.Content)
				if err := onDelta(StreamDelta{Content: choice.Delta.Content}); err != nil {
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
	if result.Model == "" {
		result.Model = model
	}
	if strings.TrimSpace(result.Content) == "" {
		return nil, fmt.Errorf("provider returned empty streamed content")
	}
	if result.FinishReason == "" {
		result.FinishReason = "stop"
	}
	return result, nil
}

type wireMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

func wireMessages(req Request) []wireMessage {
	messages := make([]wireMessage, 0, len(req.Messages)+1)
	if req.SystemPrompt != "" {
		messages = append(messages, wireMessage{Role: "system", Content: req.SystemPrompt})
	}
	for _, message := range req.Messages {
		if message.Role != "user" || len(message.ImageDataURLs) == 0 {
			messages = append(messages, wireMessage{Role: message.Role, Content: message.Content})
			continue
		}
		parts := []map[string]interface{}{{"type": "text", "text": message.Content}}
		for _, imageURL := range message.ImageDataURLs {
			parts = append(parts, map[string]interface{}{"type": "image_url", "image_url": map[string]string{"url": imageURL, "detail": "auto"}})
		}
		messages = append(messages, wireMessage{Role: message.Role, Content: parts})
	}
	return messages
}

func basePayload(req Request, model string, messages []wireMessage) map[string]interface{} {
	payload := map[string]interface{}{"model": model, "messages": messages}
	if req.MaxTokens > 0 {
		payload["max_tokens"] = req.MaxTokens
	}
	mode := "disabled"
	if req.ThinkingEnabled {
		mode = "enabled"
	}
	payload["thinking"] = map[string]string{"type": mode}
	if req.ThinkingEnabled && req.ReasoningEffort != "" {
		payload["reasoning_effort"] = req.ReasoningEffort
	}
	return payload
}

func defaultValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

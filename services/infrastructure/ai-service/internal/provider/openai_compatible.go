package provider

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type OpenAICompatible struct {
	baseURL, apiKey, model, visionModel string
	standardParameters                  bool
	modelDefaultReasoning               bool
	openCodeGo                          bool
	configuredModels                    []ModelOption
	client                              *http.Client
}

func NewOpenAICompatible(baseURL, apiKey, model, visionModel string) *OpenAICompatible {
	return &OpenAICompatible{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, model: model, visionModel: visionModel, client: &http.Client{Timeout: 600 * time.Second}}
}

// SetHTTPClient is only called while constructing an unpublished provider snapshot.
func (p *OpenAICompatible) SetHTTPClient(client *http.Client) { p.client = client }
func (p *OpenAICompatible) UseStandardParameters()            { p.standardParameters = true }
func (p *OpenAICompatible) payload(req Request, model string, messages []wireMessage) map[string]interface{} {
	result := basePayload(req, model, messages)
	if p.standardParameters || p.modelDefaultReasoning {
		delete(result, "thinking")
	}
	if p.modelDefaultReasoning || p.openCodeGo {
		delete(result, "reasoning_effort")
	}
	if p.Name() == "volcengine" && !p.modelDefaultReasoning && req.ThinkingEnabled && req.MaxTokens > 0 {
		// Ark max_tokens bounds the answer only; thinking must share the budget.
		delete(result, "max_tokens")
		result["max_completion_tokens"] = req.MaxTokens
	}
	return result
}
func (p *OpenAICompatible) Name() string {
	if p.openCodeGo {
		return OpenCodeGo
	}
	if p.configuredModels != nil {
		return "volcengine"
	}
	return "openai_compatible"
}
func (p *OpenAICompatible) Model() string { return p.model }
func (p *OpenAICompatible) ModelFor(req Request) string {
	if req.Model != "" {
		return req.Model
	}
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
	payload := p.payload(req, model, messages)
	body, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if err := p.setHeaders(httpReq, req); err != nil {
		return nil, err
	}
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("provider request failed: %w", err)
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &HTTPError{StatusCode: resp.StatusCode}
	}
	var decoded struct {
		Error   json.RawMessage `json:"error"`
		Choices []struct {
			Message      Message `json:"message"`
			FinishReason string  `json:"finish_reason"`
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
	if p.openCodeGo && len(decoded.Error) > 0 && string(decoded.Error) != "null" {
		return nil, errors.New("OpenCode Go returned a completion error")
	}
	result := &Response{PromptTokens: decoded.Usage.PromptTokens, CompletionTokens: decoded.Usage.CompletionTokens, PromptCacheHitTokens: decoded.Usage.PromptCacheHitTokens, PromptCacheMissTokens: decoded.Usage.PromptCacheMissTokens, ReasoningTokens: decoded.Usage.CompletionTokenDetails.ReasoningTokens, Model: defaultValue(decoded.Model, model)}
	if len(decoded.Choices) > 0 {
		result.Content = decoded.Choices[0].Message.Content
		result.FinishReason = decoded.Choices[0].FinishReason
	}
	if strings.TrimSpace(result.Content) == "" {
		return result, ErrEmptyContent
	}
	return result, nil
}

// CompleteDocument sends the engine's Chat Completions payload using the model's registered connection.
func (p *OpenAICompatible) CompleteDocument(ctx context.Context, body []byte, sessionID string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if err = p.setHeaders(req, Request{SessionID: sessionID}); err != nil {
		return nil, err
	}
	return p.client.Do(req)
}

func (p *OpenAICompatible) Stream(ctx context.Context, req Request, onDelta func(StreamDelta) error) (*Response, error) {
	messages := wireMessages(req)
	model := p.ModelFor(req)
	payload := p.payload(req, model, messages)
	payload["stream"] = true
	payload["stream_options"] = map[string]bool{"include_usage": true}
	body, _ := json.Marshal(payload)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	if err := p.setHeaders(httpReq, req); err != nil {
		return nil, err
	}
	httpReq.Header.Set("Accept", "text/event-stream")
	resp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("provider stream request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 2<<20))
		return nil, &HTTPError{StatusCode: resp.StatusCode}
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
			Error json.RawMessage `json:"error"`
			Usage *struct {
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
		if len(chunk.Error) > 0 && string(chunk.Error) != "null" {
			return result, fmt.Errorf("provider returned a stream error")
		}
		// Most deltas have no usage. Do not erase the last authoritative counts.
		if chunk.Usage != nil {
			result.PromptTokens = chunk.Usage.PromptTokens
			result.CompletionTokens = chunk.Usage.CompletionTokens
			result.PromptCacheHitTokens = chunk.Usage.PromptCacheHitTokens
			result.PromptCacheMissTokens = chunk.Usage.PromptCacheMissTokens
			result.ReasoningTokens = chunk.Usage.CompletionTokenDetails.ReasoningTokens
		}
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
		return result, fmt.Errorf("provider returned empty streamed content (finish_reason=%s, completion_tokens=%d, reasoning_tokens=%d)", result.FinishReason, result.CompletionTokens, result.ReasoningTokens)
	}
	if result.FinishReason == "" {
		result.FinishReason = "stop"
	}
	return result, nil
}

func (p *OpenAICompatible) setHeaders(httpReq *http.Request, req Request) error {
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	if p.openCodeGo {
		if req.SessionID == "" {
			return errors.New("OpenCode Go requires a session ID")
		}
		setOpenCodeGoHeaders(httpReq.Header, req.SessionID)
	}
	return nil
}

func setOpenCodeGoHeaders(headers http.Header, sessionID string) {
	headers.Set("User-Agent", openCodeUserAgent)
	headers.Set("x-opencode-session", sessionID)
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
	if req.JSONMode {
		payload["response_format"] = map[string]string{"type": "json_object"}
	}
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

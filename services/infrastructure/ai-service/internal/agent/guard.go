package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrUnsafeContent = errors.New("content rejected by safety policy")
	ErrInvalidInputs = errors.New("invalid structured inputs")
	ErrInputTooLong  = errors.New("input exceeds configured length")
)

type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type Field struct {
	Key      string   `json:"key"`
	Label    string   `json:"label"`
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Options  []Option `json:"options"`
}

type InputSchema struct {
	Fields []Field `json:"fields"`
}

type Guard struct {
	maxInputChars int
	blockedTerms  []string
}

func NewGuard(maxInputChars int, blockedTerms []string) *Guard {
	terms := make([]string, 0, len(blockedTerms))
	for _, term := range blockedTerms {
		if normalized := strings.ToLower(strings.TrimSpace(term)); normalized != "" {
			terms = append(terms, normalized)
		}
	}
	return &Guard{maxInputChars: maxInputChars, blockedTerms: terms}
}

func (g *Guard) Validate(schemaJSON, content string, inputs map[string]interface{}) (string, error) {
	content = strings.TrimSpace(content)
	if g.maxInputChars > 0 && utf8.RuneCountInString(content) > g.maxInputChars {
		return "", ErrInputTooLong
	}
	if err := g.ValidateOutput(content); err != nil {
		return "", err
	}

	var schema InputSchema
	if strings.TrimSpace(schemaJSON) != "" {
		if err := json.Unmarshal([]byte(schemaJSON), &schema); err != nil {
			return "", fmt.Errorf("%w: schema", ErrInvalidInputs)
		}
	}
	known := make(map[string]Field, len(schema.Fields))
	for _, field := range schema.Fields {
		known[field.Key] = field
		value, present := inputs[field.Key]
		if field.Required && (!present || isEmpty(value)) {
			return "", fmt.Errorf("%w: %s required", ErrInvalidInputs, field.Key)
		}
		if present && field.Type == "select" && !isAllowedOption(value, field.Options) {
			return "", fmt.Errorf("%w: %s option", ErrInvalidInputs, field.Key)
		}
		if present && !isEmpty(value) && !isValidFieldValue(value, field.Type) {
			return "", fmt.Errorf("%w: %s type", ErrInvalidInputs, field.Key)
		}
	}
	for key := range inputs {
		if _, ok := known[key]; !ok {
			return "", fmt.Errorf("%w: unknown field %s", ErrInvalidInputs, key)
		}
	}
	encoded, err := json.Marshal(inputs)
	if err != nil {
		return "", fmt.Errorf("%w: json", ErrInvalidInputs)
	}
	if g.maxInputChars > 0 && utf8.RuneCount(encoded) > g.maxInputChars*2 {
		return "", ErrInputTooLong
	}
	return string(encoded), nil
}

// ValidateOutput applies the server-side safety list before streamed text is persisted.
func (g *Guard) ValidateOutput(content string) error {
	joined := strings.ToLower(content)
	for _, term := range g.blockedTerms {
		if strings.Contains(joined, term) {
			return ErrUnsafeContent
		}
	}
	return nil
}

func StructuredContext(schemaJSON, inputJSON string) string {
	var schema InputSchema
	var inputs map[string]interface{}
	if json.Unmarshal([]byte(schemaJSON), &schema) != nil || json.Unmarshal([]byte(inputJSON), &inputs) != nil || len(inputs) == 0 {
		return ""
	}
	labels := make(map[string]string, len(schema.Fields))
	for _, field := range schema.Fields {
		labels[field.Key] = field.Label
	}
	keys := make([]string, 0, len(inputs))
	for key := range inputs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		label := labels[key]
		if label == "" {
			label = key
		}
		parts = append(parts, fmt.Sprintf("%s：%v", label, inputs[key]))
	}
	return "结构化资料：" + strings.Join(parts, "；")
}

func isEmpty(value interface{}) bool {
	if value == nil {
		return true
	}
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text) == ""
	}
	return false
}

func isAllowedOption(value interface{}, options []Option) bool {
	text, ok := value.(string)
	if !ok {
		return false
	}
	for _, option := range options {
		if text == option.Value {
			return true
		}
	}
	return false
}

func isValidFieldValue(value interface{}, fieldType string) bool {
	text, ok := value.(string)
	if !ok {
		return false
	}
	text = strings.TrimSpace(text)
	switch fieldType {
	case "date":
		_, err := time.Parse("2006-01-02", text)
		return err == nil
	case "time":
		_, err := time.Parse("15:04", text)
		return err == nil
	case "datetime":
		for _, layout := range []string{"2006-01-02T15:04", "2006-01-02T15:04:05", "2006-01-02 15:04", "2006-01-02 15:04:05"} {
			if _, err := time.Parse(layout, text); err == nil {
				return true
			}
		}
		return false
	case "text", "select", "":
		return utf8.RuneCountInString(text) <= 500
	default:
		return false
	}
}

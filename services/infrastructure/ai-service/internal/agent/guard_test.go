package agent

import (
	"errors"
	"strings"
	"testing"
)

func TestGuardValidatesRequiredAndOptions(t *testing.T) {
	schema := `{"fields":[{"key":"birthDate","label":"出生日期","type":"date","required":true},{"key":"gender","label":"性别","type":"select","required":true,"options":[{"value":"male","label":"男"}]}]}`
	guard := NewGuard(100, nil)
	encoded, err := guard.Validate(schema, "请分析", map[string]interface{}{"birthDate": "2000-01-01", "gender": "male"})
	if err != nil || !strings.Contains(encoded, "birthDate") {
		t.Fatalf("valid inputs rejected: %s %v", encoded, err)
	}
	if _, err := guard.Validate(schema, "请分析", map[string]interface{}{"gender": "male"}); !errors.Is(err, ErrInvalidInputs) {
		t.Fatalf("missing required field accepted: %v", err)
	}
	if _, err := guard.Validate(schema, "请分析", map[string]interface{}{"birthDate": "2000-01-01", "gender": "female"}); !errors.Is(err, ErrInvalidInputs) {
		t.Fatalf("invalid option accepted: %v", err)
	}
}

func TestGuardSafetyAndLength(t *testing.T) {
	guard := NewGuard(100, []string{"blocked"})
	if _, err := guard.Validate(`{"fields":[]}`, "contains BLOCKED", nil); !errors.Is(err, ErrUnsafeContent) {
		t.Fatalf("blocked content accepted: %v", err)
	}
	guard = NewGuard(4, nil)
	if _, err := guard.Validate(`{"fields":[]}`, "12345", nil); !errors.Is(err, ErrInputTooLong) {
		t.Fatalf("long content accepted: %v", err)
	}
}

func TestGuardRejectsUnsafeProviderOutput(t *testing.T) {
	guard := NewGuard(100, []string{"禁止输出"})
	if err := guard.ValidateOutput("模型返回了禁止输出内容"); !errors.Is(err, ErrUnsafeContent) {
		t.Fatalf("unsafe provider output accepted: %v", err)
	}
}

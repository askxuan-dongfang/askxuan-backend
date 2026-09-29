package einopoc

import (
	"errors"
	"github.com/cloudwego/eino/schema"
	"strings"
	"testing"
)

func TestInputCeilingDoesNotReplaceModelContextBudget(t *testing.T) {
	m := &budgetModel{budget: &budget{modelLimit: 4}}
	for i := 0; i < 4; i++ {
		if err := m.check([]*schema.Message{schema.UserMessage(strings.Repeat("x", 100000))}); err != nil {
			t.Fatal(err)
		}
	}
	if !errors.Is(m.check(nil), ErrBudget) {
		t.Fatal("model call budget lost")
	}
	m = &budgetModel{budget: &budget{modelLimit: 4}}
	if !errors.Is(m.check([]*schema.Message{schema.UserMessage(strings.Repeat("x", 8*1024*1024+1))}), ErrBudget) {
		t.Fatal("allocation ceiling lost")
	}
}

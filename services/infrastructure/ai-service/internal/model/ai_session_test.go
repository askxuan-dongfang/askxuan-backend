package model

import "testing"

func TestConversationTitle(t *testing.T) {
	tests := []struct {
		name     string
		question string
		want     string
	}{
		{name: "empty", question: "  ", want: "新对话"},
		{name: "trim", question: "  工作方向如何选择  ", want: "工作方向如何选择"},
		{name: "unicode limit", question: "一二三四五六七八九十一二三四五六七八九十一二三四五六七八九十一二三", want: "一二三四五六七八九十一二三四五六七八九十一二三四五六七八九十"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := conversationTitle(tt.question); got != tt.want {
				t.Fatalf("conversationTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}

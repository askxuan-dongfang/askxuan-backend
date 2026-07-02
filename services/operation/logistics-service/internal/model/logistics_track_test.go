package model

import "testing"

// ===== 物流轨迹状态机测试 =====

func TestCanTransitTrack(t *testing.T) {
	tests := []struct {
		name   string
		from   string
		to     string
		expect bool
	}{
		// 合法流转
		{"pending→in_transit", TrackStatusPending, TrackStatusInTransit, true},
		{"in_transit→delivered", TrackStatusInTransit, TrackStatusDelivered, true},
		{"delivered→signed", TrackStatusDelivered, TrackStatusSigned, true},

		// 非法流转
		{"pending→delivered(跳过运输)", TrackStatusPending, TrackStatusDelivered, false},
		{"pending→signed(跳过运输和派送)", TrackStatusPending, TrackStatusSigned, false},
		{"in_transit→signed(跳过派送)", TrackStatusInTransit, TrackStatusSigned, false},
		{"in_transit→pending(运输中不能回退)", TrackStatusInTransit, TrackStatusPending, false},
		{"delivered→in_transit(已派送不能回退)", TrackStatusDelivered, TrackStatusInTransit, false},
		{"signed→delivered(已签收不能回退)", TrackStatusSigned, TrackStatusDelivered, false},
		{"相同状态", TrackStatusPending, TrackStatusPending, false},
		{"未知源状态", "unknown", TrackStatusInTransit, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanTransitTrack(tt.from, tt.to)
			if got != tt.expect {
				t.Errorf("CanTransitTrack(%q→%q) 期望 %v, 实际 %v", tt.from, tt.to, tt.expect, got)
			}
		})
	}
}

func TestIsTrackTerminalStatus(t *testing.T) {
	// signed 是终态
	if !IsTrackTerminalStatus(TrackStatusSigned) {
		t.Error("signed 应为终态")
	}

	// 其他状态不是终态
	nonTerminal := []string{TrackStatusPending, TrackStatusInTransit, TrackStatusDelivered, "unknown"}
	for _, s := range nonTerminal {
		if IsTrackTerminalStatus(s) {
			t.Errorf("状态 %q 不应为终态", s)
		}
	}
}

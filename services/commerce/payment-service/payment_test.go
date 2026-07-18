package main

import (
	"testing"

	"github.com/askxuan/payment-service/internal/config"
)

func TestValidatePaymentConfig(t *testing.T) {
	tests := []struct {
		name     string
		env      string
		provider string
		wantErr  bool
	}{
		{name: "development mock", env: "development", provider: "mock"},
		{name: "test mock", env: "test", provider: "MOCK"},
		{name: "development unsupported", env: "development", provider: "wechat", wantErr: true},
		{name: "production reserved channel", env: "production", provider: "wechat", wantErr: true},
		{name: "production mock", env: "production", provider: "mock", wantErr: true},
		{name: "prod mock", env: "prod", provider: "MOCK", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c config.Config
			c.AppEnv = tt.env
			c.Provider = tt.provider
			if gotErr := validatePaymentConfig(c) != nil; gotErr != tt.wantErr {
				t.Fatalf("validatePaymentConfig() error=%v, wantErr=%v", gotErr, tt.wantErr)
			}
		})
	}
}

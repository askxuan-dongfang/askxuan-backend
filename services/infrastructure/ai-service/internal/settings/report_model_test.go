package settings

import (
	"testing"

	"github.com/askxuan/ai-service/internal/config"
)

func TestReportModelRemainsDeploymentOwnedWhenProviderSettingsChange(t *testing.T) {
	m, err := New(config.AIConf{Provider: "mock", Model: "mock", ReportModel: "report-model"}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	next := m.record
	next.ThinkingEnabled = true
	next.ReasoningEffort = "high"
	updated, err := m.buildSnapshot(next)
	if err != nil || updated.Config.ReportModel != "report-model" {
		t.Fatal("provider settings rebuild cleared deployment report model", err)
	}
	if m.Snapshot().Config.ThinkingEnabled || m.Snapshot().Config.ReportModel != "report-model" {
		t.Fatal("building a new provider snapshot mutated the active one")
	}
}

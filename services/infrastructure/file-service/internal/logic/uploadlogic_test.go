package logic

import (
	"testing"

	"github.com/askxuan/file-service/internal/config"
)

func TestBuildObjectURLUsesPublicBaseURL(t *testing.T) {
	got := buildObjectURL(config.MinIOConf{
		Endpoint:      "minio:9000",
		PublicBaseURL: "https://101.96.228.71/objects/",
	}, "askxuan", "temp/demo.jpg")
	if got != "https://101.96.228.71/objects/askxuan/temp/demo.jpg" {
		t.Fatalf("unexpected public object URL: %s", got)
	}
}

func TestBuildObjectURLFallsBackToEndpoint(t *testing.T) {
	got := buildObjectURL(config.MinIOConf{Endpoint: "localhost:9000"}, "askxuan", "temp/demo.jpg")
	if got != "http://localhost:9000/askxuan/temp/demo.jpg" {
		t.Fatalf("unexpected endpoint object URL: %s", got)
	}
}

package config

import "testing"

func TestMinIOConfRuntimeSeparatesInternalAndPresignTLS(t *testing.T) {
	t.Setenv("MINIO_ACCESS_KEY", "runtime-access")
	t.Setenv("MINIO_SECRET_KEY", "runtime-secret")
	t.Setenv("MEDIA_MINIO_PRESIGN_ENDPOINT", "101.96.228.71")
	t.Setenv("MEDIA_MINIO_PRESIGN_USE_SSL", "true")
	t.Setenv("MEDIA_MINIO_PUBLIC_BASE_URL", "https://101.96.228.71/askxuan-media")

	got := (MinIOConf{Endpoint: "minio:9000", UseSSL: false}).Runtime()
	if got.AccessKey != "runtime-access" || got.SecretKey != "runtime-secret" {
		t.Fatalf("runtime credentials not applied")
	}
	if got.Endpoint != "minio:9000" || got.UseSSL {
		t.Fatalf("internal endpoint was changed: %#v", got)
	}
	if got.PresignEndpoint != "101.96.228.71" || !got.PresignUseSSL {
		t.Fatalf("external presign endpoint not applied: %#v", got)
	}
	if got.PublicBaseURL != "https://101.96.228.71/askxuan-media" {
		t.Fatalf("public base URL not applied: %s", got.PublicBaseURL)
	}
}

func TestMinIOConfRuntimePrefersServiceSpecificCredentials(t *testing.T) {
	t.Setenv("MINIO_ACCESS_KEY", "shared-access")
	t.Setenv("MINIO_SECRET_KEY", "shared-secret")
	t.Setenv("MEDIA_MINIO_ACCESS_KEY", "media-access")
	t.Setenv("MEDIA_MINIO_SECRET_KEY", "media-secret")

	got := (MinIOConf{}).Runtime()
	if got.AccessKey != "media-access" || got.SecretKey != "media-secret" {
		t.Fatalf("service-specific credentials not preferred")
	}
}

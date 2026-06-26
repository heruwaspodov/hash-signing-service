package config

import (
	"testing"
	"time"
)

func TestNew_NormalizesSignerBackend(t *testing.T) {
	t.Setenv("SIGNER_BACKEND", " KMS ")

	cfg := New()

	if cfg.SignerBackend != "kms" {
		t.Fatalf("want kms, got %q", cfg.SignerBackend)
	}
}

func TestNew_LoadsGenericKMSAndAliCloudConfig(t *testing.T) {
	t.Setenv("SIGNER_BACKEND", " kms ")
	t.Setenv("KMS_PROVIDER", " ALICLOUD ")
	t.Setenv("KMS_TIMEOUT_SECONDS", "7")
	t.Setenv("ALICLOUD_KMS_MODE", " LOCAL ")
	t.Setenv("ALICLOUD_KMS_KEY_ID", "key-id")
	t.Setenv("ALICLOUD_KMS_ENDPOINT", "endpoint")
	t.Setenv("ALICLOUD_REGION_ID", "ap-southeast-5")
	t.Setenv("ALICLOUD_ACCESS_KEY_ID", "access-key")
	t.Setenv("ALICLOUD_ACCESS_KEY_SECRET", "secret")
	t.Setenv("ALICLOUD_KMS_LOCAL_SCENARIO", " THROTTLED ")

	cfg := New()

	if cfg.SignerBackend != "kms" {
		t.Fatalf("want kms, got %q", cfg.SignerBackend)
	}
	if cfg.KMS.Provider != "alicloud" {
		t.Fatalf("want alicloud, got %q", cfg.KMS.Provider)
	}
	if cfg.KMS.Timeout != 7*time.Second {
		t.Fatalf("want 7s, got %s", cfg.KMS.Timeout)
	}
	if cfg.KMS.AliCloud.Mode != "local" {
		t.Fatalf("want local, got %q", cfg.KMS.AliCloud.Mode)
	}
	if cfg.KMS.AliCloud.LocalScenario != "throttled" {
		t.Fatalf("want throttled, got %q", cfg.KMS.AliCloud.LocalScenario)
	}
}

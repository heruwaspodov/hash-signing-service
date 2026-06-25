package config

import "testing"

func TestNew_NormalizesSignerBackend(t *testing.T) {
	t.Setenv("SIGNER_BACKEND", " AWSKMS ")

	cfg := New()

	if cfg.SignerBackend != "awskms" {
		t.Fatalf("want awskms, got %q", cfg.SignerBackend)
	}
}

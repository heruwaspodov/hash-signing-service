package main

import (
	"strings"
	"testing"

	"hash-signing-service/config"
)

func TestInitSigner_RejectsUnsupportedBackend(t *testing.T) {
	cfg := config.New()
	cfg.SignerBackend = "aws-kmss"

	err := initSigner(".", cfg)
	if err == nil {
		t.Fatal("expected unsupported backend error")
	}
	if !strings.Contains(err.Error(), "unsupported SIGNER_BACKEND") {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Signer != nil {
		t.Fatal("unsupported backend must not initialize a signer")
	}
}

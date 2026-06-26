package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"hash-signing-service/config"
	"hash-signing-service/interfaces/services"
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

func TestInitSigner_AliCloudLocalKMS(t *testing.T) {
	keyPath := writeTestPrivateKey(t)
	cfg := config.New()
	cfg.AppEnvironment = "test"
	cfg.SignerBackend = "kms"
	cfg.KMS.Provider = "alicloud"
	cfg.KMS.AliCloud.Mode = services.AlibabaKMSModeLocal
	cfg.KMS.AliCloud.LocalScenario = services.AlibabaLocalScenarioSuccess
	cfg.CertPath.AppKey = keyPath

	if err := initSigner("", cfg); err != nil {
		t.Fatalf("initSigner: %v", err)
	}
	if cfg.Signer == nil {
		t.Fatal("expected signer to be initialized")
	}
	if _, ok := cfg.Signer.(*services.KMSSigner); !ok {
		t.Fatalf("want *services.KMSSigner, got %T", cfg.Signer)
	}
}

func writeTestPrivateKey(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	keyBytes := x509.MarshalPKCS1PrivateKey(key)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: keyBytes})
	path := filepath.Join(t.TempDir(), "signing.key")
	if err := os.WriteFile(path, pemBytes, 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

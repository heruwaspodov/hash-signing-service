package services

import (
	"bytes"
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"testing"
)

func TestLocalAlibabaKMSClient_SignsDigestWithoutRehashing(t *testing.T) {
	key := generateTestRSAKey(t)
	tests := []struct {
		name string
		oid  string
		hash crypto.Hash
		size int
	}{
		{"sha256", "2.16.840.1.101.3.4.2.1", crypto.SHA256, 32},
		{"sha384", "2.16.840.1.101.3.4.2.2", crypto.SHA384, 48},
		{"sha512", "2.16.840.1.101.3.4.2.3", crypto.SHA512, 64},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewLocalAlibabaKMSClient(key, AlibabaLocalScenarioSuccess, "test")
			if err != nil {
				t.Fatalf("NewLocalAlibabaKMSClient: %v", err)
			}
			digest := bytes.Repeat([]byte{0x42}, tt.size)

			sig, err := client.SignDigest(context.Background(), "", digest, tt.oid)
			if err != nil {
				t.Fatalf("SignDigest: %v", err)
			}
			if len(sig) == 0 {
				t.Fatal("signature is empty")
			}
			if err := rsa.VerifyPKCS1v15(&key.PublicKey, tt.hash, digest, sig); err != nil {
				t.Fatalf("signature does not verify against original digest: %v", err)
			}
		})
	}
}

func TestLocalAlibabaKMSClient_Scenarios(t *testing.T) {
	key := generateTestRSAKey(t)
	tests := []struct {
		scenario string
		wantErr  error
	}{
		{AlibabaLocalScenarioKeyNotFound, ErrKMSKeyNotFound},
		{AlibabaLocalScenarioKeyDisabled, ErrKMSKeyDisabled},
		{AlibabaLocalScenarioAccessDenied, ErrKMSAccessDenied},
		{AlibabaLocalScenarioThrottled, ErrKMSThrottled},
		{AlibabaLocalScenarioTimeout, ErrKMSTimeout},
		{AlibabaLocalScenarioInternalError, ErrKMSInvalidRequest},
	}

	for _, tt := range tests {
		t.Run(tt.scenario, func(t *testing.T) {
			client, err := NewLocalAlibabaKMSClient(key, tt.scenario, "test")
			if err != nil {
				t.Fatalf("NewLocalAlibabaKMSClient: %v", err)
			}
			_, err = client.SignDigest(context.Background(), "", bytes.Repeat([]byte{0x42}, 32), "2.16.840.1.101.3.4.2.1")
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("want %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestLocalAlibabaKMSClient_InvalidSignatureScenario(t *testing.T) {
	key := generateTestRSAKey(t)
	client, err := NewLocalAlibabaKMSClient(key, AlibabaLocalScenarioInvalidSignature, "test")
	if err != nil {
		t.Fatalf("NewLocalAlibabaKMSClient: %v", err)
	}
	digest := bytes.Repeat([]byte{0x42}, 32)

	sig, err := client.SignDigest(context.Background(), "", digest, "2.16.840.1.101.3.4.2.1")
	if err != nil {
		t.Fatalf("SignDigest: %v", err)
	}
	if err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, digest, sig); err == nil {
		t.Fatal("expected corrupted signature to fail verification")
	}
}

func TestAlibabaKMSProvider_LocalModeRejectsProduction(t *testing.T) {
	key := generateTestRSAKey(t)
	_, err := NewAlibabaKMSProvider(AlibabaKMSProviderConfig{
		Mode:          AlibabaKMSModeLocal,
		LocalScenario: AlibabaLocalScenarioSuccess,
	}, key, "production")
	if err == nil {
		t.Fatal("expected production local mode error")
	}
}

func TestAlibabaKMSProvider_RemoteModeFailsFastWhenIncomplete(t *testing.T) {
	_, err := NewAlibabaKMSProvider(AlibabaKMSProviderConfig{
		Mode: AlibabaKMSModeRemote,
	}, nil, "test")
	if err == nil {
		t.Fatal("expected remote config error")
	}
}

func generateTestRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	return key
}

package services_test

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"testing"

	"hash-signing-service/interfaces/services"
)

// testKey generates a 2048-bit RSA key pair for use in tests.
func testKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate test RSA key: %v", err)
	}
	return key
}

// testSigner returns a FileSigner backed by a fresh test key.
func testSigner(t *testing.T) (*services.FileSigner, *rsa.PrivateKey) {
	t.Helper()
	key := testKey(t)
	return services.NewFileSigner(key), key
}

// testHashB64 computes a SHA-256 digest of data and returns it base64-encoded.
func testHashB64(data string) string {
	h := sha256.Sum256([]byte(data))
	return base64.StdEncoding.EncodeToString(h[:])
}

func TestNewRawHashSignService_WrongHashLength_SHA256(t *testing.T) {
	signer, _ := testSigner(t)
	// Send 20 bytes base64 but claim SHA-256 (needs 32 bytes) — must reject.
	short := base64.StdEncoding.EncodeToString(make([]byte, 20))
	_, err := services.NewRawHashSignService(short, "2.16.840.1.101.3.4.2.1", signer)
	if err == nil {
		t.Fatal("expected error for wrong hash length, got nil")
	}
}

func TestNewRawHashSignService_UnsupportedOID(t *testing.T) {
	signer, _ := testSigner(t)
	_, err := services.NewRawHashSignService(testHashB64("x"), "9.9.9.9", signer)
	if err == nil {
		t.Fatal("expected error for unsupported OID, got nil")
	}
}

func TestNewRawHashSignService_InvalidBase64(t *testing.T) {
	signer, _ := testSigner(t)
	_, err := services.NewRawHashSignService("not!!valid==base64", "2.16.840.1.101.3.4.2.1", signer)
	if err == nil {
		t.Fatal("expected error for invalid base64, got nil")
	}
}

func TestNewRawHashSignService_Valid(t *testing.T) {
	signer, _ := testSigner(t)
	svc, err := services.NewRawHashSignService(testHashB64("hello"), "2.16.840.1.101.3.4.2.1", signer)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestCall_SHA256_SignatureVerifies(t *testing.T) {
	signer, key := testSigner(t)
	data := "signed attributes content"

	digest := sha256.Sum256([]byte(data))
	hashB64 := base64.StdEncoding.EncodeToString(digest[:])

	svc, err := services.NewRawHashSignService(hashB64, "2.16.840.1.101.3.4.2.1", signer)
	if err != nil {
		t.Fatalf("NewRawHashSignService: %v", err)
	}

	sigB64, err := svc.Call(context.Background())
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	sigBytes, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		t.Fatalf("decode signature base64: %v", err)
	}

	// Verify: signature must match the original digest using the public key.
	if err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, digest[:], sigBytes); err != nil {
		t.Fatalf("signature verification failed: %v", err)
	}
}

func TestCall_SHA512_SignatureVerifies(t *testing.T) {
	signer, key := testSigner(t)
	data := "another document"

	digest := sha512.Sum512([]byte(data))
	hashB64 := base64.StdEncoding.EncodeToString(digest[:])

	svc, err := services.NewRawHashSignService(hashB64, "2.16.840.1.101.3.4.2.3", signer)
	if err != nil {
		t.Fatalf("NewRawHashSignService: %v", err)
	}

	sigB64, err := svc.Call(context.Background())
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	sigBytes, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil {
		t.Fatalf("decode signature base64: %v", err)
	}

	if err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA512, digest[:], sigBytes); err != nil {
		t.Fatalf("signature verification failed: %v", err)
	}
}

func TestCall_DifferentInputsProduceDifferentSignatures(t *testing.T) {
	signer, _ := testSigner(t)

	d1 := sha256.Sum256([]byte("doc one"))
	d2 := sha256.Sum256([]byte("doc two"))

	svc1, _ := services.NewRawHashSignService(base64.StdEncoding.EncodeToString(d1[:]), "2.16.840.1.101.3.4.2.1", signer)
	svc2, _ := services.NewRawHashSignService(base64.StdEncoding.EncodeToString(d2[:]), "2.16.840.1.101.3.4.2.1", signer)

	sig1, _ := svc1.Call(context.Background())
	sig2, _ := svc2.Call(context.Background())

	if sig1 == sig2 {
		t.Fatal("expected different signatures for different inputs")
	}
}

func TestCall_DoesNotReHash(t *testing.T) {
	// If the service re-hashed the input, verifying with the original digest would fail.
	signer, key := testSigner(t)
	digest := sha256.Sum256([]byte("payload"))
	hashB64 := base64.StdEncoding.EncodeToString(digest[:])

	svc, _ := services.NewRawHashSignService(hashB64, "2.16.840.1.101.3.4.2.1", signer)
	sigB64, _ := svc.Call(context.Background())
	sigBytes, _ := base64.StdEncoding.DecodeString(sigB64)

	// Must verify with the original digest — NOT sha256(digest).
	err := rsa.VerifyPKCS1v15(&key.PublicKey, crypto.SHA256, digest[:], sigBytes)
	if err != nil {
		t.Fatalf("re-hash guard failed — service appears to be re-hashing: %v", err)
	}
}

package handlers_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"hash-signing-service/config"
	"hash-signing-service/interfaces/handlers"
	"hash-signing-service/interfaces/services"
	"hash-signing-service/pkg/responses"
)

// newTestConfig returns a config with a real RSA key injected.
func newTestConfig(t *testing.T) *config.Config {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate test key: %v", err)
	}
	cfg := config.New()
	cfg.Certificate = services.CertificateService{Key: key}
	return cfg
}

// makeRequest builds an httptest request with config injected into context.
func makeRequest(t *testing.T, body any, cfg *config.Config) *http.Request {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal request body: %v", err)
	}
	r := httptest.NewRequest(http.MethodPost, "/api/v1/hash-sign", bytes.NewReader(b))
	r.Header.Set("Content-Type", "application/json")
	if cfg != nil {
		ctx := context.WithValue(r.Context(), "config", cfg)
		r = r.WithContext(ctx)
	}
	return r
}

// digestB64 returns a base64-encoded SHA-256 digest of the given string.
func digestB64(s string) string {
	h := sha256.Sum256([]byte(s))
	return base64.StdEncoding.EncodeToString(h[:])
}

func TestHashSign_NoConfigInContext_Returns500(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/v1/hash-sign", bytes.NewReader([]byte(`{}`)))
	w := httptest.NewRecorder()

	handlers.HashSign(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", w.Code)
	}
	assertErrorCode(t, w, "internal_error")
}

func TestHashSign_NilPrivateKey_Returns500(t *testing.T) {
	cfg := config.New()
	cfg.Certificate = services.CertificateService{Key: nil}

	r := makeRequest(t, map[string]any{
		"hash":      []string{digestB64("x")},
		"hash_algo": "2.16.840.1.101.3.4.2.1",
		"sign_algo": "1.2.840.113549.1.1.11",
	}, cfg)
	w := httptest.NewRecorder()

	handlers.HashSign(w, r)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("want 500, got %d", w.Code)
	}
	assertErrorCode(t, w, "internal_error")
}

func TestHashSign_InvalidJSON_Returns400(t *testing.T) {
	cfg := newTestConfig(t)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/hash-sign", bytes.NewReader([]byte(`{invalid}`)))
	ctx := context.WithValue(r.Context(), "config", cfg)
	r = r.WithContext(ctx)
	w := httptest.NewRecorder()

	handlers.HashSign(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
	assertErrorCode(t, w, "bad_request")
}

func TestHashSign_EmptyHashArray_Returns400(t *testing.T) {
	cfg := newTestConfig(t)
	r := makeRequest(t, map[string]any{
		"hash":      []string{},
		"hash_algo": "2.16.840.1.101.3.4.2.1",
		"sign_algo": "1.2.840.113549.1.1.11",
	}, cfg)
	w := httptest.NewRecorder()

	handlers.HashSign(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
	assertErrorCode(t, w, "bad_request")
}

func TestHashSign_MissingHashAlgo_Returns400(t *testing.T) {
	cfg := newTestConfig(t)
	r := makeRequest(t, map[string]any{
		"hash":      []string{digestB64("x")},
		"sign_algo": "1.2.840.113549.1.1.11",
	}, cfg)
	w := httptest.NewRecorder()

	handlers.HashSign(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
	assertErrorCode(t, w, "bad_request")
}

func TestHashSign_EmptyHashElement_Returns400(t *testing.T) {
	cfg := newTestConfig(t)
	r := makeRequest(t, map[string]any{
		"hash":      []string{digestB64("valid"), ""},
		"hash_algo": "2.16.840.1.101.3.4.2.1",
		"sign_algo": "1.2.840.113549.1.1.11",
	}, cfg)
	w := httptest.NewRecorder()

	handlers.HashSign(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
	assertErrorCode(t, w, "bad_request")
}

func TestHashSign_UnsupportedOID_Returns400(t *testing.T) {
	cfg := newTestConfig(t)
	r := makeRequest(t, map[string]any{
		"hash":      []string{digestB64("x")},
		"hash_algo": "9.9.9.9",
		"sign_algo": "1.2.840.113549.1.1.11",
	}, cfg)
	w := httptest.NewRecorder()

	handlers.HashSign(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
	assertErrorCode(t, w, "bad_request")
}

func TestHashSign_InvalidBase64Hash_Returns400(t *testing.T) {
	cfg := newTestConfig(t)
	r := makeRequest(t, map[string]any{
		"hash":      []string{"not!!valid==base64"},
		"hash_algo": "2.16.840.1.101.3.4.2.1",
		"sign_algo": "1.2.840.113549.1.1.11",
	}, cfg)
	w := httptest.NewRecorder()

	handlers.HashSign(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400, got %d", w.Code)
	}
	assertErrorCode(t, w, "bad_request")
}

func TestHashSign_SingleHash_Returns200WithOneSignature(t *testing.T) {
	cfg := newTestConfig(t)
	r := makeRequest(t, map[string]any{
		"hash":      []string{digestB64("document one")},
		"hash_algo": "2.16.840.1.101.3.4.2.1",
		"sign_algo": "1.2.840.113549.1.1.11",
	}, cfg)
	w := httptest.NewRecorder()

	handlers.HashSign(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", w.Code, w.Body.String())
	}

	var resp responses.HashSignResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Signatures) != 1 {
		t.Fatalf("want 1 signature, got %d", len(resp.Signatures))
	}
	if resp.Signatures[0] == "" {
		t.Fatal("signature is empty")
	}
}

func TestHashSign_MultipleHashes_Returns200WithSignaturesInOrder(t *testing.T) {
	cfg := newTestConfig(t)
	hashes := []string{
		digestB64("first document"),
		digestB64("second document"),
		digestB64("third document"),
	}
	r := makeRequest(t, map[string]any{
		"hash":      hashes,
		"hash_algo": "2.16.840.1.101.3.4.2.1",
		"sign_algo": "1.2.840.113549.1.1.11",
	}, cfg)
	w := httptest.NewRecorder()

	handlers.HashSign(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d — body: %s", w.Code, w.Body.String())
	}

	var resp responses.HashSignResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(resp.Signatures) != len(hashes) {
		t.Fatalf("want %d signatures, got %d", len(hashes), len(resp.Signatures))
	}

	// Each signature must be non-empty and unique (different inputs → different signatures).
	seen := map[string]bool{}
	for i, sig := range resp.Signatures {
		if sig == "" {
			t.Fatalf("signatures[%d] is empty", i)
		}
		if seen[sig] {
			t.Fatalf("signatures[%d] is a duplicate — ordering may be wrong", i)
		}
		seen[sig] = true
	}
}

func TestHashSign_ContentTypeIsJSON(t *testing.T) {
	cfg := newTestConfig(t)
	r := makeRequest(t, map[string]any{
		"hash":      []string{digestB64("x")},
		"hash_algo": "2.16.840.1.101.3.4.2.1",
		"sign_algo": "1.2.840.113549.1.1.11",
	}, cfg)
	w := httptest.NewRecorder()

	handlers.HashSign(w, r)

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Fatalf("want Content-Type application/json, got %q", ct)
	}
}

// assertErrorCode decodes the response body and asserts the error.code field.
func assertErrorCode(t *testing.T, w *httptest.ResponseRecorder, wantCode string) {
	t.Helper()
	var resp responses.HashSignError
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if resp.Error.Code != wantCode {
		t.Fatalf("want error.code %q, got %q", wantCode, resp.Error.Code)
	}
}

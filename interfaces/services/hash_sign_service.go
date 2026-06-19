package services

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"errors"
	"fmt"
)

// HashAlgoOIDMap maps digest algorithm OIDs to their Go crypto.Hash equivalents.
// Only algorithms listed here are accepted; all others return a bad_request error.
var HashAlgoOIDMap = map[string]crypto.Hash{
	"2.16.840.1.101.3.4.2.1": crypto.SHA256, // SHA-256
	"2.16.840.1.101.3.4.2.2": crypto.SHA384, // SHA-384
	"2.16.840.1.101.3.4.2.3": crypto.SHA512, // SHA-512
}

// RawHashSignService performs RSA signing on a pre-computed digest.
// It does NOT hash the input — the caller is expected to send the final digest bytes.
type RawHashSignService struct {
	hashBytes  []byte
	hashFunc   crypto.Hash
	privateKey *rsa.PrivateKey
}

// NewRawHashSignService constructs a RawHashSignService.
// It validates the hash_algo OID and decodes the base64 digest.
// Returns an error if the OID is unsupported or the base64 is malformed.
func NewRawHashSignService(hashB64 string, hashAlgoOID string, privateKey *rsa.PrivateKey) (*RawHashSignService, error) {
	hashFunc, ok := HashAlgoOIDMap[hashAlgoOID]
	if !ok {
		return nil, fmt.Errorf("unsupported hash_algo: %s", hashAlgoOID)
	}

	hashBytes, err := base64.StdEncoding.DecodeString(hashB64)
	if err != nil {
		return nil, errors.New("invalid base64 hash")
	}

	return &RawHashSignService{
		hashBytes:  hashBytes,
		hashFunc:   hashFunc,
		privateKey: privateKey,
	}, nil
}

// Call signs the pre-hashed bytes using RSA PKCS#1 v1.5.
// The digest is passed directly to rsa.SignPKCS1v15 — Go does not re-hash
// when the hash bytes are provided explicitly.
// Returns the signature as a standard base64-encoded string.
func (s *RawHashSignService) Call() (string, error) {
	sigBytes, err := rsa.SignPKCS1v15(rand.Reader, s.privateKey, s.hashFunc, s.hashBytes)
	if err != nil {
		return "", fmt.Errorf("signing failed: %v", err)
	}
	return base64.StdEncoding.EncodeToString(sigBytes), nil
}

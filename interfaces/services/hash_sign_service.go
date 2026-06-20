package services

import (
	"encoding/base64"
	"errors"
	"fmt"
)

// RawHashSignService validates the request and delegates signing to a Signer backend.
// It is backend-agnostic: the same service works with FileSigner or HSMSigner.
type RawHashSignService struct {
	hashBytes   []byte
	hashAlgoOID string
	signer      Signer
}

// NewRawHashSignService validates the hash_algo OID and decodes the base64 digest.
// Returns an error if the OID is unsupported or the base64 is malformed.
func NewRawHashSignService(hashB64 string, hashAlgoOID string, signer Signer) (*RawHashSignService, error) {
	if _, ok := HashAlgoOIDMap[hashAlgoOID]; !ok {
		return nil, fmt.Errorf("unsupported hash_algo: %s", hashAlgoOID)
	}

	hashBytes, err := base64.StdEncoding.DecodeString(hashB64)
	if err != nil {
		return nil, errors.New("invalid base64 hash")
	}

	return &RawHashSignService{
		hashBytes:   hashBytes,
		hashAlgoOID: hashAlgoOID,
		signer:      signer,
	}, nil
}

// Call delegates to the configured Signer backend and returns the
// base64-encoded signature. The digest is passed as-is — no re-hashing.
func (s *RawHashSignService) Call() (string, error) {
	sigBytes, err := s.signer.Sign(s.hashBytes, s.hashAlgoOID)
	if err != nil {
		return "", fmt.Errorf("signing failed: %v", err)
	}
	return base64.StdEncoding.EncodeToString(sigBytes), nil
}

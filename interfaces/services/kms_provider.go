package services

import (
	"context"
	"errors"
)

// KMSProvider signs precomputed digests with a cloud-KMS-compatible backend.
type KMSProvider interface {
	SignDigest(ctx context.Context, keyID string, digest []byte, hashAlgoOID string) ([]byte, error)
	ProviderName() string
}

var (
	ErrKMSKeyNotFound    = errors.New("kms key not found")
	ErrKMSKeyDisabled    = errors.New("kms key disabled")
	ErrKMSAccessDenied   = errors.New("kms access denied")
	ErrKMSThrottled      = errors.New("kms throttled")
	ErrKMSTimeout        = errors.New("kms timeout")
	ErrKMSInvalidRequest = errors.New("kms invalid request")
)

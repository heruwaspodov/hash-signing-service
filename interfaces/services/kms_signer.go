package services

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// KMSSigner delegates digest signing to a configured KMSProvider.
type KMSSigner struct {
	provider KMSProvider
	keyID    string
	timeout  time.Duration
}

// NewKMSSigner creates a provider-based KMS signer.
func NewKMSSigner(provider KMSProvider, keyID string, timeout time.Duration) *KMSSigner {
	return &KMSSigner{
		provider: provider,
		keyID:    strings.TrimSpace(keyID),
		timeout:  timeout,
	}
}

// Sign passes the already-validated digest unchanged to the provider.
func (k *KMSSigner) Sign(ctx context.Context, hashBytes []byte, hashAlgoOID string) ([]byte, error) {
	if _, ok := HashAlgoOIDMap[hashAlgoOID]; !ok {
		return nil, fmt.Errorf("unsupported hash_algo OID: %s", hashAlgoOID)
	}

	if k.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, k.timeout)
		defer cancel()
	}

	return k.provider.SignDigest(ctx, k.keyID, hashBytes, hashAlgoOID)
}

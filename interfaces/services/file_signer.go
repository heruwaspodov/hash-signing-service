package services

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
)

// FileSigner signs using an RSA private key loaded from a PEM file.
// Intended for local development. SIGNER_BACKEND=file (default).
type FileSigner struct {
	privateKey *rsa.PrivateKey
}

// NewFileSigner creates a FileSigner with the given RSA private key.
func NewFileSigner(key *rsa.PrivateKey) *FileSigner {
	return &FileSigner{privateKey: key}
}

// Sign performs RSA PKCS#1 v1.5 signing using the in-memory private key.
// hashBytes are raw digest bytes — not re-hashed inside this method.
// Go's rsa.SignPKCS1v15 automatically prepends the DigestInfo ASN.1 prefix.
func (f *FileSigner) Sign(ctx context.Context, hashBytes []byte, hashAlgoOID string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	hashFunc, ok := HashAlgoOIDMap[hashAlgoOID]
	if !ok {
		return nil, fmt.Errorf("unsupported hash_algo: %s", hashAlgoOID)
	}
	return rsa.SignPKCS1v15(rand.Reader, f.privateKey, hashFunc, hashBytes)
}

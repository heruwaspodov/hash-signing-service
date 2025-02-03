package services

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"errors"
	"fmt"
	"hash-signing-service/interfaces/utils"
	"log"
)

type PathCertificateService struct {
	AppCert   string
	AppKey    string
	AppSubCA  string
	AppRootCA string
}

type CertificateService struct {
	Key    *rsa.PrivateKey
	Cert   *x509.Certificate
	SubCA  *x509.Certificate
	RootCA *x509.Certificate
}

type HashSigningService struct {
	digest     crypto.Hash
	hash       string
	privateKey *rsa.PrivateKey
	secretKey  string
}

func NewHashSigningService(digest crypto.Hash, hash string, privateKey *rsa.PrivateKey, secretKey string) *HashSigningService {
	return &HashSigningService{
		digest:     digest,
		hash:       hash,
		privateKey: privateKey,
		secretKey:  secretKey,
	}
}

// Call executes the signing process
func (h *HashSigningService) Call() (string, error) {
	signature, err := h.signing()
	if err != nil {
		return "", err
	}
	return signature, nil
}

func (h *HashSigningService) signing() (string, error) {
	// Get raw signature bytes
	signatureBytes, err := h.signingProcess()
	if err != nil {
		return "", err
	}

	// Encrypt the raw signature bytes
	encryptedSignature, err := encryptWithSecretKey(h.secretKey, string(signatureBytes))
	if err != nil {
		return "", err
	}

	log.Printf("Encrypted Signature: %q\n", encryptedSignature)
	return encryptedSignature, nil
}

func encryptWithSecretKey(secretKey string, data string) (string, error) {
	key := []byte(secretKey)
	return utils.EncryptAES(key, data)
}

func decryptWithSecretKey(secretKey string, data string) (string, error) {
	key := []byte(secretKey)
	return utils.DecryptAES(key, data)
}

func (h *HashSigningService) signingProcess() ([]byte, error) {
	// Validasi private key
	if h.privateKey == nil {
		return nil, errors.New("private key is nil")
	}

	// Log the received hash and key info
	log.Printf("Received encrypted hash: %s", h.hash)

	// Decrypt the hash
	decryptedHash, err := decryptWithSecretKey(h.secretKey, h.hash)
	if err != nil {
		return nil, fmt.Errorf("hash decryption failed: %v", err)
	}

	log.Printf("Successfully decrypted hash (hex): %x", []byte(decryptedHash))

	// Validasi digest
	if !h.digest.Available() {
		return nil, errors.New("unsupported hash function")
	}

	// Create hash for signing
	hasher := h.digest.New()
	if _, err := hasher.Write([]byte(decryptedHash)); err != nil {
		return nil, fmt.Errorf("hash computation failed: %v", err)
	}
	hashed := hasher.Sum(nil)

	// Sign the hash
	signature, err := rsa.SignPKCS1v15(rand.Reader, h.privateKey, h.digest, hashed)

	if err != nil {
		return nil, fmt.Errorf("signing error: %v", err)
	}

	log.Printf("Raw signature bytes: %x\n", signature)

	return signature, nil
}

package services

import "crypto"

// Signer is the backend-agnostic interface for RSA signing.
// Implementations: FileSigner (PEM key on disk), HSMSigner (PKCS#11 hardware/software HSM).
// Switch between them via SIGNER_BACKEND env — handler and service layer are unaffected.
type Signer interface {
	Sign(hashBytes []byte, hashAlgoOID string) ([]byte, error)
}

// HashAlgoOIDMap maps digest algorithm OIDs to Go crypto.Hash values.
// Used by FileSigner for rsa.SignPKCS1v15 and by hash_sign_service for OID validation.
var HashAlgoOIDMap = map[string]crypto.Hash{
	"2.16.840.1.101.3.4.2.1": crypto.SHA256, // SHA-256
	"2.16.840.1.101.3.4.2.2": crypto.SHA384, // SHA-384
	"2.16.840.1.101.3.4.2.3": crypto.SHA512, // SHA-512
}

// expectedHashLen maps digest OIDs to the required digest byte length.
// Validates that the caller actually sent a digest of the right algorithm,
// not a truncated or wrong-algorithm payload that HSM would silently sign.
var expectedHashLen = map[string]int{
	"2.16.840.1.101.3.4.2.1": 32, // SHA-256
	"2.16.840.1.101.3.4.2.2": 48, // SHA-384
	"2.16.840.1.101.3.4.2.3": 64, // SHA-512
}

// digestInfoPrefix maps digest OIDs to their ASN.1 DigestInfo prefix bytes.
// Required for CKM_RSA_PKCS: HSM expects DigestInfo || hash, not raw hash bytes.
// rsa.SignPKCS1v15 (Go stdlib) prepends this automatically — PKCS#11 does not.
var digestInfoPrefix = map[string][]byte{
	"2.16.840.1.101.3.4.2.1": {0x30, 0x31, 0x30, 0x0d, 0x06, 0x09, 0x60, 0x86, 0x48, 0x01, 0x65, 0x03, 0x04, 0x02, 0x01, 0x05, 0x00, 0x04, 0x20},
	"2.16.840.1.101.3.4.2.2": {0x30, 0x41, 0x30, 0x0d, 0x06, 0x09, 0x60, 0x86, 0x48, 0x01, 0x65, 0x03, 0x04, 0x02, 0x02, 0x05, 0x00, 0x04, 0x30},
	"2.16.840.1.101.3.4.2.3": {0x30, 0x51, 0x30, 0x0d, 0x06, 0x09, 0x60, 0x86, 0x48, 0x01, 0x65, 0x03, 0x04, 0x02, 0x03, 0x05, 0x00, 0x04, 0x40},
}

package services

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
)

// KMSSigner signs via AWS KMS using RSASSA_PKCS1_V1_5.
// KMS handles DigestInfo + PKCS#1 v1.5 padding internally —
// unlike HSMSigner (CKM_RSA_PKCS), do NOT prepend DigestInfo before calling Sign.
// SIGNER_BACKEND=awskms.
type KMSSigner struct {
	client *kms.Client
	keyID  string
}

// NewKMSSigner initializes an AWS KMS client and validates the key exists.
func NewKMSSigner(region, keyID string) (*KMSSigner, error) {
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %v", err)
	}

	client := kms.NewFromConfig(cfg)

	// Verify key is accessible at startup.
	if _, err := client.DescribeKey(context.Background(), &kms.DescribeKeyInput{
		KeyId: aws.String(keyID),
	}); err != nil {
		return nil, fmt.Errorf("kms describe key %q: %v", keyID, err)
	}

	return &KMSSigner{client: client, keyID: keyID}, nil
}

// Sign calls KMS Sign with MessageType=DIGEST — passes raw hash bytes,
// KMS handles PKCS#1 v1.5 DigestInfo wrapping internally.
func (k *KMSSigner) Sign(hashBytes []byte, hashAlgoOID string) ([]byte, error) {
	algo, ok := kmsAlgoMap[hashAlgoOID]
	if !ok {
		return nil, fmt.Errorf("unsupported hash_algo OID: %s", hashAlgoOID)
	}

	out, err := k.client.Sign(context.Background(), &kms.SignInput{
		KeyId:            aws.String(k.keyID),
		Message:          hashBytes,
		MessageType:      types.MessageTypeDigest,
		SigningAlgorithm: algo,
	})
	if err != nil {
		return nil, fmt.Errorf("kms sign: %v", err)
	}

	return out.Signature, nil
}

// kmsAlgoMap maps digest OIDs to KMS RSASSA_PKCS1_V1_5 algorithm specs.
var kmsAlgoMap = map[string]types.SigningAlgorithmSpec{
	"2.16.840.1.101.3.4.2.1": types.SigningAlgorithmSpecRsassaPkcs1V15Sha256,
	"2.16.840.1.101.3.4.2.2": types.SigningAlgorithmSpecRsassaPkcs1V15Sha384,
	"2.16.840.1.101.3.4.2.3": types.SigningAlgorithmSpecRsassaPkcs1V15Sha512,
}

package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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
	client kmsAPI
	keyID  string
}

type kmsAPI interface {
	DescribeKey(context.Context, *kms.DescribeKeyInput, ...func(*kms.Options)) (*kms.DescribeKeyOutput, error)
	Sign(context.Context, *kms.SignInput, ...func(*kms.Options)) (*kms.SignOutput, error)
}

// NewKMSSigner initializes an AWS KMS client and validates the key exists.
func NewKMSSigner(region, keyID string) (*KMSSigner, error) {
	region = strings.TrimSpace(region)
	keyID = strings.TrimSpace(keyID)
	if region == "" {
		return nil, errors.New("AWS_KMS_REGION is required")
	}
	if keyID == "" {
		return nil, errors.New("AWS_KMS_KEY_ID is required when SIGNER_BACKEND=awskms")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %v", err)
	}

	client := kms.NewFromConfig(cfg)

	if err := validateKMSKey(ctx, client, keyID); err != nil {
		return nil, err
	}

	return &KMSSigner{client: client, keyID: keyID}, nil
}

func newKMSSignerWithClient(client kmsAPI, keyID string) *KMSSigner {
	return &KMSSigner{client: client, keyID: keyID}
}

// Sign calls KMS Sign with MessageType=DIGEST — passes raw hash bytes,
// KMS handles PKCS#1 v1.5 DigestInfo wrapping internally.
func (k *KMSSigner) Sign(ctx context.Context, hashBytes []byte, hashAlgoOID string) ([]byte, error) {
	algo, ok := kmsAlgoMap[hashAlgoOID]
	if !ok {
		return nil, fmt.Errorf("unsupported hash_algo OID: %s", hashAlgoOID)
	}

	out, err := k.client.Sign(ctx, &kms.SignInput{
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

func validateKMSKey(ctx context.Context, client kmsAPI, keyID string) error {
	out, err := client.DescribeKey(ctx, &kms.DescribeKeyInput{
		KeyId: aws.String(keyID),
	})
	if err != nil {
		return fmt.Errorf("kms describe key %q: %v", keyID, err)
	}
	if out.KeyMetadata == nil {
		return fmt.Errorf("kms describe key %q: missing key metadata", keyID)
	}

	metadata := out.KeyMetadata
	if metadata.KeyState != types.KeyStateEnabled {
		return fmt.Errorf("KMS key %q is not enabled: %s", keyID, metadata.KeyState)
	}
	if metadata.KeyUsage != types.KeyUsageTypeSignVerify {
		return fmt.Errorf("KMS key %q must have KeyUsage=SIGN_VERIFY", keyID)
	}

	switch metadata.KeySpec {
	case types.KeySpecRsa2048, types.KeySpecRsa3072, types.KeySpecRsa4096:
	default:
		return fmt.Errorf("KMS key %q must be an RSA signing key; got %s", keyID, metadata.KeySpec)
	}

	if !containsSigningAlgorithm(metadata.SigningAlgorithms, types.SigningAlgorithmSpecRsassaPkcs1V15Sha256) {
		return fmt.Errorf("KMS key %q does not allow RSASSA_PKCS1_V1_5_SHA_256", keyID)
	}

	return nil
}

func containsSigningAlgorithm(algos []types.SigningAlgorithmSpec, want types.SigningAlgorithmSpec) bool {
	for _, algo := range algos {
		if algo == want {
			return true
		}
	}
	return false
}

// kmsAlgoMap maps digest OIDs to KMS RSASSA_PKCS1_V1_5 algorithm specs.
var kmsAlgoMap = map[string]types.SigningAlgorithmSpec{
	"2.16.840.1.101.3.4.2.1": types.SigningAlgorithmSpecRsassaPkcs1V15Sha256,
	"2.16.840.1.101.3.4.2.2": types.SigningAlgorithmSpecRsassaPkcs1V15Sha384,
	"2.16.840.1.101.3.4.2.3": types.SigningAlgorithmSpecRsassaPkcs1V15Sha512,
}

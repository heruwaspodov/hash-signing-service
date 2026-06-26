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

// AWSKMSProvider signs via AWS KMS using RSASSA_PKCS1_V1_5.
type AWSKMSProvider struct {
	client kmsAPI
}

type kmsAPI interface {
	DescribeKey(context.Context, *kms.DescribeKeyInput, ...func(*kms.Options)) (*kms.DescribeKeyOutput, error)
	Sign(context.Context, *kms.SignInput, ...func(*kms.Options)) (*kms.SignOutput, error)
}

func NewAWSKMSProvider(region, keyID string) (*AWSKMSProvider, error) {
	region = strings.TrimSpace(region)
	keyID = strings.TrimSpace(keyID)
	if region == "" {
		return nil, errors.New("AWS_KMS_REGION is required")
	}
	if keyID == "" {
		return nil, errors.New("AWS_KMS_KEY_ID is required when SIGNER_BACKEND=kms and KMS_PROVIDER=aws")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load aws config: %v", err)
	}

	client := kms.NewFromConfig(cfg)
	if err := validateAWSKMSKey(ctx, client, keyID); err != nil {
		return nil, err
	}

	return &AWSKMSProvider{client: client}, nil
}

func newAWSKMSProviderWithClient(client kmsAPI) *AWSKMSProvider {
	return &AWSKMSProvider{client: client}
}

func (a *AWSKMSProvider) ProviderName() string {
	return "aws"
}

// SignDigest calls AWS KMS Sign with MessageType=DIGEST, so KMS handles
// PKCS#1 v1.5 DigestInfo wrapping internally.
func (a *AWSKMSProvider) SignDigest(ctx context.Context, keyID string, digest []byte, hashAlgoOID string) ([]byte, error) {
	algo, ok := awsKMSAlgoMap[hashAlgoOID]
	if !ok {
		return nil, fmt.Errorf("unsupported hash_algo OID: %s", hashAlgoOID)
	}

	out, err := a.client.Sign(ctx, &kms.SignInput{
		KeyId:            aws.String(keyID),
		Message:          digest,
		MessageType:      types.MessageTypeDigest,
		SigningAlgorithm: algo,
	})
	if err != nil {
		return nil, fmt.Errorf("kms sign: %v", err)
	}

	return out.Signature, nil
}

func validateAWSKMSKey(ctx context.Context, client kmsAPI, keyID string) error {
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

	if !containsAWSSigningAlgorithm(metadata.SigningAlgorithms, types.SigningAlgorithmSpecRsassaPkcs1V15Sha256) {
		return fmt.Errorf("KMS key %q does not allow RSASSA_PKCS1_V1_5_SHA_256", keyID)
	}

	return nil
}

func containsAWSSigningAlgorithm(algos []types.SigningAlgorithmSpec, want types.SigningAlgorithmSpec) bool {
	for _, algo := range algos {
		if algo == want {
			return true
		}
	}
	return false
}

var awsKMSAlgoMap = map[string]types.SigningAlgorithmSpec{
	"2.16.840.1.101.3.4.2.1": types.SigningAlgorithmSpecRsassaPkcs1V15Sha256,
	"2.16.840.1.101.3.4.2.2": types.SigningAlgorithmSpecRsassaPkcs1V15Sha384,
	"2.16.840.1.101.3.4.2.3": types.SigningAlgorithmSpecRsassaPkcs1V15Sha512,
}

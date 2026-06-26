package services

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"strings"
)

const (
	AlibabaLocalScenarioSuccess          = "success"
	AlibabaLocalScenarioKeyNotFound      = "key_not_found"
	AlibabaLocalScenarioKeyDisabled      = "key_disabled"
	AlibabaLocalScenarioAccessDenied     = "access_denied"
	AlibabaLocalScenarioThrottled        = "throttled"
	AlibabaLocalScenarioTimeout          = "timeout"
	AlibabaLocalScenarioInternalError    = "internal_error"
	AlibabaLocalScenarioInvalidSignature = "invalid_signature"
)

type LocalAlibabaKMSClient struct {
	privateKey *rsa.PrivateKey
	scenario   string
}

func NewLocalAlibabaKMSClient(privateKey *rsa.PrivateKey, scenario, appEnv string) (*LocalAlibabaKMSClient, error) {
	if privateKey == nil {
		return nil, errors.New("local alicloud kms requires RSA private key")
	}

	scenario = strings.ToLower(strings.TrimSpace(scenario))
	if scenario == "" {
		scenario = AlibabaLocalScenarioSuccess
	}
	if scenario != AlibabaLocalScenarioSuccess && strings.ToLower(strings.TrimSpace(appEnv)) == "production" {
		return nil, errors.New("ALICLOUD_KMS_LOCAL_SCENARIO other than success is not allowed in production")
	}
	if !isSupportedAlibabaLocalScenario(scenario) {
		return nil, fmt.Errorf("unsupported ALICLOUD_KMS_LOCAL_SCENARIO=%q", scenario)
	}

	return &LocalAlibabaKMSClient{
		privateKey: privateKey,
		scenario:   scenario,
	}, nil
}

func (l *LocalAlibabaKMSClient) SignDigest(ctx context.Context, _ string, digest []byte, hashAlgoOID string) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	switch l.scenario {
	case AlibabaLocalScenarioKeyNotFound:
		return nil, ErrKMSKeyNotFound
	case AlibabaLocalScenarioKeyDisabled:
		return nil, ErrKMSKeyDisabled
	case AlibabaLocalScenarioAccessDenied:
		return nil, ErrKMSAccessDenied
	case AlibabaLocalScenarioThrottled:
		return nil, ErrKMSThrottled
	case AlibabaLocalScenarioTimeout:
		return nil, ErrKMSTimeout
	case AlibabaLocalScenarioInternalError:
		return nil, fmt.Errorf("alicloud kms local internal error: %w", ErrKMSInvalidRequest)
	}

	hashFunc, ok := HashAlgoOIDMap[hashAlgoOID]
	if !ok {
		return nil, fmt.Errorf("unsupported hash_algo OID: %s", hashAlgoOID)
	}

	sig, err := rsa.SignPKCS1v15(rand.Reader, l.privateKey, hashFunc, digest)
	if err != nil {
		return nil, err
	}
	if l.scenario == AlibabaLocalScenarioInvalidSignature && len(sig) > 0 {
		sig[0] ^= 0xff
	}

	return sig, nil
}

func isSupportedAlibabaLocalScenario(scenario string) bool {
	switch scenario {
	case AlibabaLocalScenarioSuccess,
		AlibabaLocalScenarioKeyNotFound,
		AlibabaLocalScenarioKeyDisabled,
		AlibabaLocalScenarioAccessDenied,
		AlibabaLocalScenarioThrottled,
		AlibabaLocalScenarioTimeout,
		AlibabaLocalScenarioInternalError,
		AlibabaLocalScenarioInvalidSignature:
		return true
	default:
		return false
	}
}

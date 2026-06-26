package services

import (
	"context"
	"crypto/rsa"
	"errors"
	"fmt"
	"strings"
)

const (
	AlibabaKMSModeLocal  = "local"
	AlibabaKMSModeRemote = "remote"
)

// AlibabaKMSProviderConfig carries Alibaba-specific KMS settings without
// coupling the services package to the application config package.
type AlibabaKMSProviderConfig struct {
	Mode            string
	KeyID           string
	Endpoint        string
	RegionID        string
	AccessKeyID     string
	AccessKeySecret string
	LocalScenario   string
}

type AlibabaKMSProvider struct {
	mode   string
	local  *LocalAlibabaKMSClient
	remote *RealAlibabaKMSClient
}

func NewAlibabaKMSProvider(cfg AlibabaKMSProviderConfig, privateKey *rsa.PrivateKey, appEnv string) (KMSProvider, error) {
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	switch mode {
	case AlibabaKMSModeLocal:
		if !isAlibabaLocalEnvAllowed(appEnv) {
			return nil, errors.New("ALICLOUD_KMS_MODE=local is allowed only when APP_ENV=local or APP_ENV=test")
		}
		if privateKey == nil {
			return nil, errors.New("CERT_KEY_FILE must parse as RSA private key when ALICLOUD_KMS_MODE=local")
		}
		local, err := NewLocalAlibabaKMSClient(privateKey, cfg.LocalScenario, appEnv)
		if err != nil {
			return nil, err
		}
		return &AlibabaKMSProvider{mode: mode, local: local}, nil

	case AlibabaKMSModeRemote:
		remote, err := NewRealAlibabaKMSClient(cfg)
		if err != nil {
			return nil, err
		}
		return &AlibabaKMSProvider{mode: mode, remote: remote}, nil

	default:
		return nil, fmt.Errorf("unsupported ALICLOUD_KMS_MODE=%q; supported values: local, remote", cfg.Mode)
	}
}

func (a *AlibabaKMSProvider) ProviderName() string {
	return "alicloud"
}

func (a *AlibabaKMSProvider) SignDigest(ctx context.Context, keyID string, digest []byte, hashAlgoOID string) ([]byte, error) {
	switch a.mode {
	case AlibabaKMSModeLocal:
		return a.local.SignDigest(ctx, keyID, digest, hashAlgoOID)
	case AlibabaKMSModeRemote:
		return a.remote.SignDigest(ctx, keyID, digest, hashAlgoOID)
	default:
		return nil, fmt.Errorf("unsupported ALICLOUD_KMS_MODE=%q", a.mode)
	}
}

func isAlibabaLocalEnvAllowed(appEnv string) bool {
	switch strings.ToLower(strings.TrimSpace(appEnv)) {
	case "local", "test":
		return true
	default:
		return false
	}
}

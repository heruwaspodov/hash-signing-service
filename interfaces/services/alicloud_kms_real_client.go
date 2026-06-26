package services

import (
	"context"
	"errors"
	"strings"
)

type RealAlibabaKMSClient struct {
	cfg AlibabaKMSProviderConfig
}

func NewRealAlibabaKMSClient(cfg AlibabaKMSProviderConfig) (*RealAlibabaKMSClient, error) {
	if strings.TrimSpace(cfg.KeyID) == "" {
		return nil, errors.New("ALICLOUD_KMS_KEY_ID is required when ALICLOUD_KMS_MODE=remote")
	}
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil, errors.New("ALICLOUD_KMS_ENDPOINT is required when ALICLOUD_KMS_MODE=remote")
	}
	if strings.TrimSpace(cfg.AccessKeyID) == "" {
		return nil, errors.New("ALICLOUD_ACCESS_KEY_ID is required when ALICLOUD_KMS_MODE=remote")
	}
	if strings.TrimSpace(cfg.AccessKeySecret) == "" {
		return nil, errors.New("ALICLOUD_ACCESS_KEY_SECRET is required when ALICLOUD_KMS_MODE=remote")
	}

	return nil, errors.New("ALICLOUD_KMS_MODE=remote is not implemented until the exact Alibaba KMS asymmetric sign API contract is verified")
}

func (r *RealAlibabaKMSClient) SignDigest(context.Context, string, []byte, string) ([]byte, error) {
	return nil, errors.New("alicloud kms remote signing is not implemented")
}

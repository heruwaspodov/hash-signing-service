package services

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/kms/types"
)

type fakeKMSClient struct {
	describeOut *kms.DescribeKeyOutput
	describeErr error
	signOut     *kms.SignOutput
	signErr     error
	signInput   *kms.SignInput
	signCalls   int
}

func (f *fakeKMSClient) DescribeKey(context.Context, *kms.DescribeKeyInput, ...func(*kms.Options)) (*kms.DescribeKeyOutput, error) {
	return f.describeOut, f.describeErr
}

func (f *fakeKMSClient) Sign(_ context.Context, in *kms.SignInput, _ ...func(*kms.Options)) (*kms.SignOutput, error) {
	f.signCalls++
	f.signInput = in
	return f.signOut, f.signErr
}

func validKMSMetadata() *kms.DescribeKeyOutput {
	return &kms.DescribeKeyOutput{
		KeyMetadata: &types.KeyMetadata{
			KeyState: types.KeyStateEnabled,
			KeyUsage: types.KeyUsageTypeSignVerify,
			KeySpec:  types.KeySpecRsa3072,
			SigningAlgorithms: []types.SigningAlgorithmSpec{
				types.SigningAlgorithmSpecRsassaPkcs1V15Sha256,
				types.SigningAlgorithmSpecRsassaPkcs1V15Sha384,
				types.SigningAlgorithmSpecRsassaPkcs1V15Sha512,
			},
		},
	}
}

func TestKMSSignerSign_UsesDigestMessageTypeAndRawDigest(t *testing.T) {
	client := &fakeKMSClient{
		signOut: &kms.SignOutput{Signature: []byte("signature")},
	}
	provider := newAWSKMSProviderWithClient(client)
	signer := NewKMSSigner(provider, "alias/test", 0)
	digest := bytes.Repeat([]byte{0xab}, 32)

	sig, err := signer.Sign(context.Background(), digest, "2.16.840.1.101.3.4.2.1")
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	if string(sig) != "signature" {
		t.Fatalf("unexpected signature %q", sig)
	}
	if client.signInput.MessageType != types.MessageTypeDigest {
		t.Fatalf("want MessageType DIGEST, got %s", client.signInput.MessageType)
	}
	if client.signInput.SigningAlgorithm != types.SigningAlgorithmSpecRsassaPkcs1V15Sha256 {
		t.Fatalf("want SHA256 signing algorithm, got %s", client.signInput.SigningAlgorithm)
	}
	if !bytes.Equal(client.signInput.Message, digest) {
		t.Fatal("KMS message does not match raw digest bytes")
	}
}

func TestKMSSignerSign_MapsSHA384AndSHA512(t *testing.T) {
	tests := []struct {
		name string
		oid  string
		want types.SigningAlgorithmSpec
	}{
		{"sha384", "2.16.840.1.101.3.4.2.2", types.SigningAlgorithmSpecRsassaPkcs1V15Sha384},
		{"sha512", "2.16.840.1.101.3.4.2.3", types.SigningAlgorithmSpecRsassaPkcs1V15Sha512},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &fakeKMSClient{signOut: &kms.SignOutput{Signature: []byte("ok")}}
			provider := newAWSKMSProviderWithClient(client)
			signer := NewKMSSigner(provider, "alias/test", 0)

			if _, err := signer.Sign(context.Background(), []byte("digest"), tt.oid); err != nil {
				t.Fatalf("Sign: %v", err)
			}
			if client.signInput.SigningAlgorithm != tt.want {
				t.Fatalf("want %s, got %s", tt.want, client.signInput.SigningAlgorithm)
			}
		})
	}
}

func TestKMSSignerSign_UnsupportedOIDDoesNotCallKMS(t *testing.T) {
	client := &fakeKMSClient{}
	provider := newAWSKMSProviderWithClient(client)
	signer := NewKMSSigner(provider, "alias/test", 0)

	if _, err := signer.Sign(context.Background(), []byte("digest"), "9.9.9.9"); err == nil {
		t.Fatal("expected unsupported OID error")
	}
	if client.signCalls != 0 {
		t.Fatalf("want no KMS calls, got %d", client.signCalls)
	}
}

func TestKMSSignerSign_ReturnsKMSError(t *testing.T) {
	client := &fakeKMSClient{signErr: errors.New("boom")}
	provider := newAWSKMSProviderWithClient(client)
	signer := NewKMSSigner(provider, "alias/test", 0)

	if _, err := signer.Sign(context.Background(), []byte("digest"), "2.16.840.1.101.3.4.2.1"); err == nil {
		t.Fatal("expected KMS error")
	}
}

func TestValidateKMSKey_AcceptsValidMetadata(t *testing.T) {
	client := &fakeKMSClient{describeOut: validKMSMetadata()}

	if err := validateAWSKMSKey(context.Background(), client, "alias/test"); err != nil {
		t.Fatalf("validateAWSKMSKey: %v", err)
	}
}

func TestValidateKMSKey_RejectsInvalidMetadata(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*types.KeyMetadata)
	}{
		{"disabled", func(m *types.KeyMetadata) { m.KeyState = types.KeyStateDisabled }},
		{"wrong usage", func(m *types.KeyMetadata) { m.KeyUsage = types.KeyUsageTypeEncryptDecrypt }},
		{"non RSA", func(m *types.KeyMetadata) { m.KeySpec = types.KeySpecEccNistP256 }},
		{"missing sha256 algorithm", func(m *types.KeyMetadata) {
			m.SigningAlgorithms = []types.SigningAlgorithmSpec{types.SigningAlgorithmSpecRsassaPkcs1V15Sha384}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := validKMSMetadata()
			tt.mutate(out.KeyMetadata)
			client := &fakeKMSClient{describeOut: out}

			if err := validateAWSKMSKey(context.Background(), client, "alias/test"); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestNewAWSKMSProvider_RejectsBlankInputs(t *testing.T) {
	if _, err := NewAWSKMSProvider("", "alias/test"); err == nil {
		t.Fatal("expected blank region error")
	}
	if _, err := NewAWSKMSProvider("ap-southeast-1", " "); err == nil {
		t.Fatal("expected blank key ID error")
	}
}

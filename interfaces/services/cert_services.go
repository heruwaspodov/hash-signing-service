package services

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io/ioutil"
)

type CertData struct {
	Key    *rsa.PrivateKey
	Cert   *x509.Certificate
	SubCA  *x509.Certificate
	RootCA *x509.Certificate
}

var Cert CertData

func LoadKey(filePath string) (*rsa.PrivateKey, error) {
	// Read the private key file
	keyData, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	// Decode the PEM block
	block, _ := pem.Decode(keyData)
	if block == nil || (block.Type != "PRIVATE KEY" && block.Type != "RSA PRIVATE KEY") {
		return nil, errors.New("failed to decode PEM block containing private key")
	}

	// Try to parse the key as PKCS#1
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}

	// If PKCS#1 parsing fails, try PKCS#8
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("failed to parse private key as PKCS#1 or PKCS#8")
	}

	// Ensure the key is of type *rsa.PrivateKey
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("private key is not an RSA key")
	}

	return rsaKey, nil
}

func LoadCert(filePath string) (*x509.Certificate, error) {
	certData, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(certData)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("failed to decode PEM block containing certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	return cert, nil
}

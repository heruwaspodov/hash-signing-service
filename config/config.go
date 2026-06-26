package config

import (
	"hash-signing-service/interfaces/services"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config represent config keys.
type Config struct {
	AppEnvironment string
	AppLanguage    string
	AppPort        string
	AppTimezone    string
	AppSecretKey   string
	EnableCors     bool
	EnableLogger   bool
	CertPath       services.PathCertificateService
	Certificate    services.CertificateService

	// SignerBackend selects the signing backend: "file" (default), "pkcs11", or "kms".
	SignerBackend string
	// HSM holds connection config for the PKCS#11 backend.
	HSM HSMConfig
	// KMS holds connection config for provider-based KMS backends.
	KMS KMSConfig
	// Signer is the initialized signing backend, set by main after startup.
	Signer services.Signer
}

// HSMConfig holds PKCS#11 connection parameters.
// Used when SIGNER_BACKEND=pkcs11.
type HSMConfig struct {
	ModulePath string // path to libsofthsm2.so or cloud HSM PKCS#11 library
	TokenLabel string // CKA_LABEL of the token
	PIN        string // user PIN
	KeyLabel   string // CKA_LABEL of the private key object
	KeyID      string // CKA_ID of the private key (hex string, e.g. "01"); used alongside KeyLabel to avoid duplicate-label ambiguity
}

// KMSConfig holds provider-based KMS connection parameters.
// Used when SIGNER_BACKEND=kms.
type KMSConfig struct {
	Provider string
	Timeout  time.Duration
	AWS      AWSKMSConfig
	AliCloud AlibabaKMSConfig
}

// AWSKMSConfig holds AWS KMS connection parameters.
type AWSKMSConfig struct {
	Region string
	KeyID  string
}

// AlibabaKMSConfig holds Alibaba Cloud KMS connection parameters.
type AlibabaKMSConfig struct {
	Mode            string
	KeyID           string
	Endpoint        string
	RegionID        string
	AccessKeyID     string
	AccessKeySecret string
	LocalScenario   string
}

func New() *Config {
	return &Config{
		AppEnvironment: getEnv("APP_ENV", "local"),
		AppLanguage:    getEnv("APP_LANG", "en"),
		AppPort:        getEnv("APP_PORT", "7777"),
		AppTimezone:    getEnv("APP_TIMEZONE", "Asia/Jakarta"),
		AppSecretKey:   getEnv("APP_SECRET_KEY", "Asia/Jakarta"),
		EnableCors:     getEnvAsBool("ENABLE_CORS", true),
		EnableLogger:   getEnvAsBool("ENABLE_LOGGER", true),
		CertPath: services.PathCertificateService{
			AppCert:   getEnv("CERT_FILE", "certs/signing.crt"),
			AppKey:    getEnv("CERT_KEY_FILE", "certs/signing.key"),
			AppSubCA:  getEnv("CERT_SUB_CA_FILE", "certs/sub-ca.crt"),
			AppRootCA: getEnv("CERT_ROOT_CA_FILE", "certs/root-ca.crt"),
		},
		SignerBackend: strings.ToLower(strings.TrimSpace(getEnv("SIGNER_BACKEND", "file"))),
		HSM: HSMConfig{
			ModulePath: getEnv("HSM_MODULE_PATH", "/usr/lib/softhsm/libsofthsm2.so"),
			TokenLabel: getEnv("HSM_TOKEN_LABEL", ""),
			PIN:        getEnv("HSM_PIN", ""),
			KeyLabel:   getEnv("HSM_KEY_LABEL", ""),
			KeyID:      getEnv("HSM_KEY_ID", ""),
		},
		KMS: KMSConfig{
			Provider: strings.ToLower(strings.TrimSpace(getEnv("KMS_PROVIDER", ""))),
			Timeout:  time.Duration(getEnvAsInt("KMS_TIMEOUT_SECONDS", 10)) * time.Second,
			AWS: AWSKMSConfig{
				Region: getEnv("AWS_KMS_REGION", "ap-southeast-1"),
				KeyID:  getEnv("AWS_KMS_KEY_ID", ""),
			},
			AliCloud: AlibabaKMSConfig{
				Mode:            strings.ToLower(strings.TrimSpace(getEnv("ALICLOUD_KMS_MODE", ""))),
				KeyID:           getEnv("ALICLOUD_KMS_KEY_ID", ""),
				Endpoint:        getEnv("ALICLOUD_KMS_ENDPOINT", ""),
				RegionID:        getEnv("ALICLOUD_REGION_ID", ""),
				AccessKeyID:     getEnv("ALICLOUD_ACCESS_KEY_ID", ""),
				AccessKeySecret: getEnv("ALICLOUD_ACCESS_KEY_SECRET", ""),
				LocalScenario:   strings.ToLower(strings.TrimSpace(getEnv("ALICLOUD_KMS_LOCAL_SCENARIO", "success"))),
			},
		},
	}
}

// Simple helper function to read an environment or return a default value.
func getEnv(key string, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}

	if nextValue := os.Getenv(key); nextValue != "" {
		return nextValue
	}

	return defaultVal
}

// Helper to read an environment variable into a bool or return default value.
func getEnvAsBool(name string, defaultVal bool) bool {
	valStr := getEnv(name, "")
	if val, err := strconv.ParseBool(valStr); err == nil {
		return val
	}

	return defaultVal
}

// Helper to read an environment variable into an int or return default value.
func getEnvAsInt(name string, defaultVal int) int {
	valStr := getEnv(name, "")
	if val, err := strconv.Atoi(valStr); err == nil {
		return val
	}

	return defaultVal
}

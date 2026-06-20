package config

import (
	"hash-signing-service/interfaces/services"
	"os"
	"strconv"
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

	// SignerBackend selects the signing backend: "file" (default) or "pkcs11".
	SignerBackend string
	// HSM holds connection config for the PKCS#11 backend.
	HSM    HSMConfig
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
		SignerBackend: getEnv("SIGNER_BACKEND", "file"),
		HSM: HSMConfig{
			ModulePath: getEnv("HSM_MODULE_PATH", "/usr/lib/softhsm/libsofthsm2.so"),
			TokenLabel: getEnv("HSM_TOKEN_LABEL", ""),
			PIN:        getEnv("HSM_PIN", ""),
			KeyLabel:   getEnv("HSM_KEY_LABEL", ""),
			KeyID:      getEnv("HSM_KEY_ID", ""),
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

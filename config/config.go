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

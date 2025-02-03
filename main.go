package main

import (
	"crypto/rsa"
	"crypto/x509"
	"hash-signing-service/config"
	"hash-signing-service/interfaces/routes"
	"hash-signing-service/interfaces/services"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

func init() {
	// load .env file
	err := godotenv.Load(".env")

	if err != nil {
		log.Fatalf("Error loading .env file")
	}
}

func main() {
	// Init Config
	config := config.New()

	basePath, errBasePath := os.Getwd()
	if errBasePath != nil {
		log.Fatalf("Error getting current directory: %v", errBasePath)
	}

	// Init Key
	key, errorKey := initKeyFile(basePath, config)

	if errorKey != nil {
		log.Fatalf("Error loading Key certificate: %v", errorKey)
		os.Exit(1)
	}

	config.Certificate.Key = key

	// Init Cert
	cert, errorCert := initCertFile(basePath, config)

	if errorCert != nil {
		log.Fatalf("Error loading Cert certificate: %v", errorCert)
		os.Exit(1)
	}

	config.Certificate.Cert = cert

	// Init SubCA
	subCA, errorSubCA := initCertSubCAFile(basePath, config)

	if errorSubCA != nil {
		log.Fatalf("Error loading SubCA certificate: %v", errorSubCA)
		os.Exit(1)
	}

	config.Certificate.SubCA = subCA

	// Init RootCA
	rootCA, errorRootCA := initCertRootCAFile(basePath, config)
	if errorRootCA != nil {
		log.Fatalf("Error loading SubCA certificate: %v", errorSubCA)
		os.Exit(1)
	}

	config.Certificate.RootCA = rootCA

	// Init Router
	router := routes.New(config).Init()

	// Init Server
	server := &http.Server{
		Addr:    ":" + config.AppPort,
		Handler: router,
	}

	if config.Certificate.Key == nil {
		log.Fatal("Private key failed to load")
	}

	log.Printf("Private key loaded successfully: %v", config.Certificate.Key != nil)

	log.Println("Server is starting on :" + config.AppPort + "...")

	err := server.ListenAndServe()
	if err != nil {
		log.Fatal("Error starting the server: ", err)
		os.Exit(1)
	}
}

func initKeyFile(basePath string, config *config.Config) (*rsa.PrivateKey, error) {
	keyFile := filepath.Join(basePath, config.CertPath.AppKey)

	cert, errLoadKeyFile := services.LoadKey(keyFile)
	if errLoadKeyFile != nil {
		return nil, errLoadKeyFile
	}

	return cert, nil
}

func initCertFile(basePath string, config *config.Config) (*x509.Certificate, error) {
	certFile := filepath.Join(basePath, config.CertPath.AppCert)

	cert, errLoadCertFile := services.LoadCert(certFile)
	if errLoadCertFile != nil {
		return nil, errLoadCertFile
	}

	return cert, nil
}

func initCertSubCAFile(basePath string, config *config.Config) (*x509.Certificate, error) {
	certFile := filepath.Join(basePath, config.CertPath.AppSubCA)

	cert, errLoadCertFile := services.LoadCert(certFile)
	if errLoadCertFile != nil {
		return nil, errLoadCertFile
	}

	return cert, nil
}

func initCertRootCAFile(basePath string, config *config.Config) (*x509.Certificate, error) {
	certFile := filepath.Join(basePath, config.CertPath.AppRootCA)

	cert, errLoadCertFile := services.LoadCert(certFile)
	if errLoadCertFile != nil {
		return nil, errLoadCertFile
	}

	return cert, nil
}

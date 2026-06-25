package main

import (
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"

	"hash-signing-service/config"
	"hash-signing-service/interfaces/routes"
	"hash-signing-service/interfaces/services"
)

func init() {
	// .env is optional — in Docker/Kubernetes env vars are injected directly.
	_ = godotenv.Load(".env")
}

func main() {
	cfg := config.New()

	basePath, err := os.Getwd()
	if err != nil {
		log.Fatalf("Error getting current directory: %v", err)
	}

	// Init Signer — backend selected by SIGNER_BACKEND env.
	if err := initSigner(basePath, cfg); err != nil {
		log.Fatal(err)
	}
	if hsmSigner, ok := cfg.Signer.(*services.HSMSigner); ok {
		defer hsmSigner.Close()
	}

	// Load certificate chain (used by msign-backend for CMS embedding reference).
	cert, err := initCertFile(basePath, cfg)
	if err != nil {
		log.Fatalf("Error loading signing cert: %v", err)
	}
	cfg.Certificate.Cert = cert

	subCA, err := initCertSubCAFile(basePath, cfg)
	if err != nil {
		log.Fatalf("Error loading sub-CA cert: %v", err)
	}
	cfg.Certificate.SubCA = subCA

	rootCA, err := initCertRootCAFile(basePath, cfg)
	if err != nil {
		log.Fatalf("Error loading root-CA cert: %v", err)
	}
	cfg.Certificate.RootCA = rootCA

	// Init Router & Server.
	router := routes.New(cfg).Init()
	server := &http.Server{
		Addr:    ":" + cfg.AppPort,
		Handler: router,
	}

	log.Println("Server is starting on :" + cfg.AppPort + "...")

	if err := server.ListenAndServe(); err != nil {
		log.Fatal("Error starting the server: ", err)
	}
}

func initSigner(basePath string, cfg *config.Config) error {
	switch cfg.SignerBackend {
	case "file":
		key, err := initKeyFile(basePath, cfg)
		if err != nil {
			return err
		}
		cfg.Certificate.Key = key
		cfg.Signer = services.NewFileSigner(key)
		log.Print("Signer backend: file")
		return nil

	case "pkcs11":
		hsmSigner, err := services.NewHSMSigner(
			cfg.HSM.ModulePath,
			cfg.HSM.TokenLabel,
			cfg.HSM.PIN,
			cfg.HSM.KeyLabel,
			cfg.HSM.KeyID,
		)
		if err != nil {
			return err
		}
		cfg.Signer = hsmSigner
		log.Printf("Signer backend: pkcs11 (module: %s, token: %s, key: %s)",
			cfg.HSM.ModulePath, cfg.HSM.TokenLabel, cfg.HSM.KeyLabel)
		return nil

	case "awskms":
		kmsSigner, err := services.NewKMSSigner(cfg.KMS.Region, cfg.KMS.KeyID)
		if err != nil {
			return err
		}
		cfg.Signer = kmsSigner
		log.Printf("Signer backend: awskms (region: %s, key: %s)", cfg.KMS.Region, cfg.KMS.KeyID)
		return nil

	default:
		return fmt.Errorf("unsupported SIGNER_BACKEND=%q; supported values: file, pkcs11, awskms", cfg.SignerBackend)
	}
}

func initKeyFile(basePath string, cfg *config.Config) (*rsa.PrivateKey, error) {
	return services.LoadKey(filepath.Join(basePath, cfg.CertPath.AppKey))
}

func initCertFile(basePath string, cfg *config.Config) (*x509.Certificate, error) {
	return services.LoadCert(filepath.Join(basePath, cfg.CertPath.AppCert))
}

func initCertSubCAFile(basePath string, cfg *config.Config) (*x509.Certificate, error) {
	return services.LoadCert(filepath.Join(basePath, cfg.CertPath.AppSubCA))
}

func initCertRootCAFile(basePath string, cfg *config.Config) (*x509.Certificate, error) {
	return services.LoadCert(filepath.Join(basePath, cfg.CertPath.AppRootCA))
}

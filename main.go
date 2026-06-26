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

	case "kms":
		kmsSigner, err := initProviderKMSSigner(basePath, cfg)
		if err != nil {
			return err
		}
		cfg.Signer = kmsSigner
		log.Printf("Signer backend: kms (provider: %s)", cfg.KMS.Provider)
		return nil

	default:
		return fmt.Errorf("unsupported SIGNER_BACKEND=%q; supported values: file, pkcs11, kms", cfg.SignerBackend)
	}
}

func initAWSKMSSigner(cfg *config.Config) (*services.KMSSigner, error) {
	provider, err := services.NewAWSKMSProvider(cfg.KMS.AWS.Region, cfg.KMS.AWS.KeyID)
	if err != nil {
		return nil, err
	}
	return services.NewKMSSigner(provider, cfg.KMS.AWS.KeyID, cfg.KMS.Timeout), nil
}

func initProviderKMSSigner(basePath string, cfg *config.Config) (*services.KMSSigner, error) {
	switch cfg.KMS.Provider {
	case "aws":
		return initAWSKMSSigner(cfg)

	case "alicloud":
		var privateKey *rsa.PrivateKey
		if cfg.KMS.AliCloud.Mode == services.AlibabaKMSModeLocal {
			var err error
			privateKey, err = initKeyFile(basePath, cfg)
			if err != nil {
				return nil, err
			}
		}
		provider, err := services.NewAlibabaKMSProvider(services.AlibabaKMSProviderConfig{
			Mode:            cfg.KMS.AliCloud.Mode,
			KeyID:           cfg.KMS.AliCloud.KeyID,
			Endpoint:        cfg.KMS.AliCloud.Endpoint,
			RegionID:        cfg.KMS.AliCloud.RegionID,
			AccessKeyID:     cfg.KMS.AliCloud.AccessKeyID,
			AccessKeySecret: cfg.KMS.AliCloud.AccessKeySecret,
			LocalScenario:   cfg.KMS.AliCloud.LocalScenario,
		}, privateKey, cfg.AppEnvironment)
		if err != nil {
			return nil, err
		}
		return services.NewKMSSigner(provider, cfg.KMS.AliCloud.KeyID, cfg.KMS.Timeout), nil

	default:
		return nil, fmt.Errorf("unsupported KMS_PROVIDER=%q; supported values: aws, alicloud", cfg.KMS.Provider)
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

package handlers

import (
	"crypto"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"hash-signing-service/config"
	"hash-signing-service/interfaces/services"
	"hash-signing-service/pkg/requests"
	"hash-signing-service/pkg/responses"
)

func Signing(w http.ResponseWriter, r *http.Request) {
	cfg, ok := r.Context().Value("config").(*config.Config)
	if !ok {
		http.Error(w, "Configuration not found in context", http.StatusInternalServerError)
		return
	}

	// Validasi private key
	if cfg.Certificate.Key == nil {
		http.Error(w, "Private key not initialized", http.StatusInternalServerError)
		return
	}

	// Validasi method
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Baca body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	println(string(body))

	// Parse request
	var request requests.RequestSigning

	if err := json.Unmarshal([]byte(body), &request); err != nil {
		http.Error(w, "Invalid JSON format", http.StatusBadRequest)
		return
	}

	// Validasi request
	if request.Hash == "" {
		http.Error(w, "Hash is required", http.StatusBadRequest)
		return
	}

	// Buat service
	digestAlgo := defineDigest(request.Digest)

	// Log untuk debugging
	log.Printf("Using digest: %v", digestAlgo)
	log.Printf("Hash to sign: %s", request.Hash)

	service := services.NewHashSigningService(
		digestAlgo,
		request.Hash,
		cfg.Certificate.Key,
		cfg.AppSecretKey,
	)

	// Sign
	signature, err := service.Call()
	if err != nil {
		log.Printf("Signing error: %v", err)
		http.Error(w, fmt.Sprintf("Failed to sign hash: %v", err), http.StatusInternalServerError)
		return
	}

	response := responses.ResponseData{
		Data: responses.SignedHash{
			SignedHash: signature,
		},
	}

	// Send response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func defineDigest(s string) crypto.Hash {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "SHA256":
		return crypto.SHA256
	case "SHA512":
		return crypto.SHA512
	default:
		return crypto.SHA256
	}
}

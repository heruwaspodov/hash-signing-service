package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"hash-signing-service/config"
	"hash-signing-service/interfaces/services"
	"hash-signing-service/pkg/requests"
	"hash-signing-service/pkg/responses"
)

// HashSign handles POST /api/v1/hash-sign.
// It receives one or more pre-computed base64 digests, signs each with the RSA private key
// using PKCS#1 v1.5 padding, and returns base64-encoded signatures in the same order.
// Digests must NOT be re-hashed — the caller (msign-backend) is responsible
// for computing them before sending here.
func HashSign(w http.ResponseWriter, r *http.Request) {
	cfg, ok := r.Context().Value("config").(*config.Config)
	if !ok {
		renderHashSignError(w, http.StatusInternalServerError, "internal_error", "configuration not found in context")
		return
	}

	if cfg.Certificate.Key == nil {
		renderHashSignError(w, http.StatusInternalServerError, "internal_error", "private key not initialized")
		return
	}

	var req requests.RequestHashSign
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		renderHashSignError(w, http.StatusBadRequest, "bad_request", "invalid JSON format")
		return
	}
	defer r.Body.Close()

	if len(req.Hash) == 0 || req.HashAlgo == "" {
		renderHashSignError(w, http.StatusBadRequest, "bad_request", "missing required params")
		return
	}

	// Sign each hash in order, preserving index so signatures[i] corresponds to hash[i].
	signatures := make([]string, 0, len(req.Hash))
	for i, h := range req.Hash {
		if h == "" {
			renderHashSignError(w, http.StatusBadRequest, "bad_request", fmt.Sprintf("hash[%d] is empty", i))
			return
		}

		// NewRawHashSignService validates the hash_algo OID and decodes the base64 digest.
		svc, err := services.NewRawHashSignService(h, req.HashAlgo, cfg.Certificate.Key)
		if err != nil {
			renderHashSignError(w, http.StatusBadRequest, "bad_request", err.Error())
			return
		}

		signature, err := svc.Call()
		if err != nil {
			log.Printf("HashSign error at index %d: %v", i, err)
			renderHashSignError(w, http.StatusInternalServerError, "internal_error", "signing failed")
			return
		}

		signatures = append(signatures, signature)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(responses.HashSignResponse{
		Signatures: signatures,
	})
}

// renderHashSignError writes a JSON error body conforming to the spec error contract:
// {"error": {"code": "<machine_code>", "message": "<human msg>"}}.
func renderHashSignError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(responses.HashSignError{
		Error: responses.HashSignErrorBody{Code: code, Message: message},
	})
}

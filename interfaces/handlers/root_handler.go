package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"hash-signing-service/pkg/responses"
)

func RootHandler(w http.ResponseWriter, _ *http.Request) {
	healthResponse := responses.ResponseHealt{
		Status: "ok",
	}

	responseJSON, err := json.Marshal(healthResponse)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	_, err = w.Write(responseJSON)
	if err != nil {
		log.Fatal("Error writing JSON response: ", err)
	}
}

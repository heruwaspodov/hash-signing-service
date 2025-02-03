package responses

import (
	"encoding/json"
	"log"
	"net/http"
)

type ResponseHealt struct {
	Status string `json:"status"`
}

type ResponseError struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

type ResponseData struct {
	Data interface{} `json:"data"`
}

func RespondWithError(w http.ResponseWriter, statusCode int, message string, code string) {
	w.WriteHeader(statusCode)
	w.Header().Set("Content-Type", "application/json")
	errorResponse := ResponseError{Message: message, Code: code}
	responseJSON, err := json.Marshal(errorResponse)
	if err != nil {
		log.Println("Error marshaling JSON response:", err)
		return
	}

	_, err = w.Write(responseJSON)
	if err != nil {
		log.Println("Error writing JSON response:", err)
	}
}

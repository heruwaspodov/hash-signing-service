package responses

type HashSignResponse struct {
	Signatures []string `json:"signatures"`
}

type HashSignError struct {
	Error HashSignErrorBody `json:"error"`
}

type HashSignErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

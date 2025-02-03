package requests

type RequestSigning struct {
	Digest string `json:"digest"` // e.g., "SHA256"
	Hash   string `json:"hash"`   // Base64-encoded hash to be signed
}

package requests

type RequestHashSign struct {
	Hash     []string `json:"hash"`
	HashAlgo string   `json:"hash_algo"`
	SignAlgo string   `json:"sign_algo"`
}

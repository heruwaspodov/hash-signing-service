package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"log"
)

func EncryptAES(key []byte, plaintext string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	// Buat nonce (IV) sepanjang 12 byte
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// Buat AES-GCM cipher mode
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Encrypt plaintext dengan AES-GCM
	ciphertextWithTag := aesgcm.Seal(nil, nonce, []byte(plaintext), nil)

	// Pisahkan tag (16 byte) dan ciphertext
	tag := ciphertextWithTag[len(ciphertextWithTag)-16:]
	ciphertext := ciphertextWithTag[:len(ciphertextWithTag)-16]

	log.Printf("Enc-Nonce: %x\n", nonce)
	log.Printf("Enc-Tag: %x\n", tag)
	log.Printf("Enc-Chipertext: %x\n", ciphertext)

	// Gabungkan nonce, tag, dan ciphertext
	fullData := append(nonce, tag...)
	fullData = append(fullData, ciphertext...)

	// Ubah hasil menjadi hex
	hexData := hex.EncodeToString(fullData)

	return hexData, nil
}

func DecryptAES(key []byte, hexData string) (string, error) {
	// Decode hex menjadi byte slice
	data, err := hex.DecodeString(hexData)
	if err != nil {
		return "", err
	}

	if len(data) < 28 {
		return "", errors.New("invalid data")
	}

	// Ambil nonce, tag, dan ciphertext
	nonce := data[:12]      // 12 byte IV
	tag := data[12:28]      // 16 byte Authentication Tag
	ciphertext := data[28:] // Sisanya adalah Ciphertext

	log.Printf("Dec-Nonce: %x\n", nonce)
	log.Printf("Dec-Tag: %x\n", tag)
	log.Printf("Dec-Chipertext: %x\n", ciphertext)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Dekripsi data
	plaintext, err := aesgcm.Open(nil, nonce, append(ciphertext, tag...), nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

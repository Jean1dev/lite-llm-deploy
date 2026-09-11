package migrate

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/nacl/secretbox"
)

func DecryptCredential(value, salt string) (string, error) {
	if salt == "" {
		return "", fmt.Errorf("cipher salt not set")
	}
	key := sha256.Sum256([]byte(salt))
	if strings.HasPrefix(value, "v2:gcm:") {
		return decryptGCM(value, key[:])
	}
	return decryptSecretBox(value, key)
}

func decryptGCM(value string, key []byte) (string, error) {
	parts := strings.Split(value, ":")
	if len(parts) != 4 {
		return "", fmt.Errorf("invalid gcm format")
	}
	nonce, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return "", fmt.Errorf("gcm nonce: %w", err)
	}
	box, err := base64.StdEncoding.DecodeString(parts[3])
	if err != nil {
		return "", fmt.Errorf("gcm ciphertext: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("aes: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm: %w", err)
	}
	open, err := aead.Open(nil, nonce, box, nil)
	if err != nil {
		return "", fmt.Errorf("open gcm: %w", err)
	}
	return string(open), nil
}

func decryptSecretBox(value string, key [32]byte) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", fmt.Errorf("secretbox base64: %w", err)
	}
	if len(raw) < 24 {
		return "", fmt.Errorf("secretbox too short")
	}
	var nonce [24]byte
	copy(nonce[:], raw[:24])
	open, ok := secretbox.Open(nil, raw[24:], &nonce, &key)
	if !ok {
		return "", fmt.Errorf("open secretbox")
	}
	return string(open), nil
}

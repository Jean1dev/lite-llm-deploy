package migrate

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"testing"

	"golang.org/x/crypto/nacl/secretbox"
)

func TestDecryptCredentialLiteLLMFormats(t *testing.T) {
	const salt = "litellm-salt-test"
	const secret = "sk-provider-secret"
	key := sha256.Sum256([]byte(salt))

	t.Run("legacy secretbox", func(t *testing.T) {
		var nonce [24]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			t.Fatal(err)
		}
		sealed := secretbox.Seal(nonce[:], []byte(secret), &nonce, &key)
		ciphered := base64.StdEncoding.EncodeToString(sealed)
		got, err := DecryptCredential(ciphered, salt)
		if err != nil {
			t.Fatalf("decrypt: %v", err)
		}
		if got != secret {
			t.Errorf("got %q", got)
		}
	})

	t.Run("v2 gcm", func(t *testing.T) {
		block, err := aes.NewCipher(key[:])
		if err != nil {
			t.Fatal(err)
		}
		aead, err := cipher.NewGCM(block)
		if err != nil {
			t.Fatal(err)
		}
		nonce := make([]byte, aead.NonceSize())
		if _, err := rand.Read(nonce); err != nil {
			t.Fatal(err)
		}
		box := aead.Seal(nil, nonce, []byte(secret), nil)
		ciphered := "v2:gcm:" + base64.StdEncoding.EncodeToString(nonce) + ":" + base64.StdEncoding.EncodeToString(box)
		got, err := DecryptCredential(ciphered, salt)
		if err != nil {
			t.Fatalf("decrypt: %v", err)
		}
		if got != secret {
			t.Errorf("got %q", got)
		}
	})
}

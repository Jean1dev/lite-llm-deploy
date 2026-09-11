package key

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"
	"testing"
)

func TestHashTokenMatchesLiteLLMScheme(t *testing.T) {
	const plain = "sk-litellm-known-key-value"
	sum := sha256.Sum256([]byte(plain))
	want := hex.EncodeToString(sum[:])

	if got := HashToken(plain); got != want {
		t.Errorf("hash = %s, want %s", got, want)
	}
}

func TestGenerateFormatAndHash(t *testing.T) {
	plain, hash, err := Generate()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if !strings.HasPrefix(plain, "sk-") {
		t.Fatalf("key missing sk- prefix: %s", plain)
	}
	body := strings.TrimPrefix(plain, "sk-")
	if len(body) != 22 {
		t.Fatalf("body = %d chars, want 22", len(body))
	}
	if _, err := base64.RawURLEncoding.DecodeString(body); err != nil {
		t.Fatalf("body is not url-safe base64: %v", err)
	}
	if hash != HashToken(plain) {
		t.Errorf("returned hash does not match sha256 of the key")
	}
}

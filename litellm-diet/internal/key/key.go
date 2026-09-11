package key

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"
)

const keyPrefix = "sk-"

type Key struct {
	Hash           string
	Alias          string
	Models         []string
	Spend          float64
	MaxBudget      *float64
	BudgetDuration string
	BudgetResetAt  *time.Time
	LastActive     *time.Time
	ExpiresAt      *time.Time
	Blocked        bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func HashToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}

func Generate() (plain string, hash string, err error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate key: %w", err)
	}
	plain = keyPrefix + base64.RawURLEncoding.EncodeToString(raw)
	return plain, HashToken(plain), nil
}

package key

import (
	"fmt"
	"strings"
	"time"
)

var (
	ErrKeyNotFound     = fmt.Errorf("key not found")
	ErrKeyBlocked      = fmt.Errorf("key blocked")
	ErrKeyExpired      = fmt.Errorf("key expired")
	ErrBudgetExceeded  = fmt.Errorf("budget exceeded")
	ErrModelNotAllowed = fmt.Errorf("model not allowed")
)

func Authenticate(k Key, now time.Time) error {
	if k.Blocked {
		return ErrKeyBlocked
	}
	if k.ExpiresAt != nil && !k.ExpiresAt.After(now) {
		return ErrKeyExpired
	}
	k = ApplyReset(k, now)
	if BudgetExceeded(k) {
		return ErrBudgetExceeded
	}
	return nil
}

func AuthorizeModel(k Key, model string) error {
	if len(k.Models) == 0 {
		return nil
	}
	for _, allowed := range k.Models {
		if allowed == model {
			return nil
		}
		if strings.HasSuffix(allowed, "/*") {
			prefix := strings.TrimSuffix(allowed, "*")
			if strings.HasPrefix(model, prefix) {
				return nil
			}
		}
	}
	return fmt.Errorf("%s: %w", model, ErrModelNotAllowed)
}

func BudgetExceeded(k Key) bool {
	if k.MaxBudget == nil {
		return false
	}
	return k.Spend >= *k.MaxBudget
}

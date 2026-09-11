package key

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
)

func ApplyReset(k Key, now time.Time) Key {
	if k.BudgetDuration == "" || k.BudgetResetAt == nil {
		return k
	}
	if !now.After(*k.BudgetResetAt) {
		return k
	}

	d, err := ParseDuration(k.BudgetDuration)
	if err != nil {
		return k
	}

	next := *k.BudgetResetAt
	for !next.After(now) {
		next = next.Add(d)
	}
	k.Spend = 0
	k.BudgetResetAt = &next
	return k
}

func NextReset(now time.Time, duration string) (time.Time, error) {
	d, err := ParseDuration(duration)
	if err != nil {
		return time.Time{}, err
	}
	return now.Add(d), nil
}

func ParseDuration(value string) (time.Duration, error) {
	text := strings.TrimSpace(strings.ToLower(value))
	if text == "" {
		return 0, fmt.Errorf("empty duration")
	}

	end := 0
	for end < len(text) && (unicode.IsDigit(rune(text[end])) || text[end] == '.') {
		end++
	}
	if end == 0 || end == len(text) {
		return 0, fmt.Errorf("duration %q invalid", value)
	}

	qty, err := strconv.ParseFloat(text[:end], 64)
	if err != nil {
		return 0, fmt.Errorf("duration %q invalid: %w", value, err)
	}
	if qty <= 0 {
		return 0, fmt.Errorf("duration %q must be positive", value)
	}

	switch text[end:] {
	case "s":
		return time.Duration(qty * float64(time.Second)), nil
	case "m":
		return time.Duration(qty * float64(time.Minute)), nil
	case "h":
		return time.Duration(qty * float64(time.Hour)), nil
	case "d":
		return time.Duration(qty * float64(24*time.Hour)), nil
	case "w":
		return time.Duration(qty * float64(7*24*time.Hour)), nil
	case "mo":
		return time.Duration(qty * float64(30*24*time.Hour)), nil
	default:
		return 0, fmt.Errorf("duration %q has unknown unit", value)
	}
}

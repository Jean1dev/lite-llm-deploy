package config

import (
	"fmt"
	"strconv"
	"strings"
)

var sizeSuffixes = []struct {
	name   string
	factor int64
}{
	{"TiB", 1 << 40},
	{"GiB", 1 << 30},
	{"MiB", 1 << 20},
	{"KiB", 1 << 10},
	{"B", 1},
}

func ParseSize(value string) (int64, error) {
	text := strings.TrimSpace(value)
	if text == "" {
		return 0, fmt.Errorf("empty size")
	}

	number := text
	var factor int64 = 1
	for _, s := range sizeSuffixes {
		if strings.HasSuffix(text, s.name) {
			number = strings.TrimSpace(strings.TrimSuffix(text, s.name))
			factor = s.factor
			break
		}
	}

	qty, err := strconv.ParseInt(number, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("size %q invalid: %w", value, err)
	}
	if qty <= 0 {
		return 0, fmt.Errorf("size %q must be positive", value)
	}
	if qty > (1<<62)/factor {
		return 0, fmt.Errorf("size %q exceeds representable limit", value)
	}
	return qty * factor, nil
}

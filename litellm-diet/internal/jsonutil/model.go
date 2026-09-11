package jsonutil

import (
	"bytes"
	"fmt"
	"unicode"
)

var ErrFieldMissing = fmt.Errorf("field missing")

func ExtractModel(body []byte) (string, error) {
	return extractTopString(body, "model")
}

func extractTopString(body []byte, field string) (string, error) {
	i := skipSpace(body, 0)
	if i >= len(body) || body[i] != '{' {
		return "", fmt.Errorf("body is not a json object")
	}
	i++
	for i < len(body) {
		i = skipSpace(body, i)
		if i < len(body) && body[i] == '}' {
			return "", fmt.Errorf("model: %w", ErrFieldMissing)
		}
		match, next, err := keyEquals(body, i, field)
		if err != nil {
			return "", err
		}
		i = skipSpace(body, next)
		if i >= len(body) || body[i] != ':' {
			return "", fmt.Errorf("invalid json after key")
		}
		i = skipSpace(body, i+1)
		if match {
			if i >= len(body) || body[i] != '"' {
				return "", fmt.Errorf("field %s is not a string", field)
			}
			value, _, err := readString(body, i)
			return value, err
		}
		i, err = skipValue(body, i)
		if err != nil {
			return "", err
		}
		i = skipSpace(body, i)
		if i < len(body) && body[i] == ',' {
			i++
		}
	}
	return "", fmt.Errorf("model: %w", ErrFieldMissing)
}

func keyEquals(body []byte, i int, field string) (bool, int, error) {
	if i >= len(body) || body[i] != '"' {
		return false, 0, fmt.Errorf("invalid json key")
	}
	i++
	n := 0
	match := true
	for i < len(body) {
		c := body[i]
		if c == '\\' {
			match = false
			i += 2
			n++
			continue
		}
		if c == '"' {
			if match && n != len(field) {
				match = false
			}
			return match, i + 1, nil
		}
		if match && (n >= len(field) || c != field[n]) {
			match = false
		}
		n++
		i++
	}
	return false, 0, fmt.Errorf("unterminated json string")
}

func readString(body []byte, i int) (string, int, error) {
	if i >= len(body) || body[i] != '"' {
		return "", 0, fmt.Errorf("invalid json string")
	}
	start := i
	i++
	for i < len(body) {
		c := body[i]
		if c == '\\' {
			i += 2
			continue
		}
		if c == '"' {
			var s string
			if err := unmarshalString(body[start:i+1], &s); err != nil {
				return "", 0, err
			}
			return s, i + 1, nil
		}
		i++
	}
	return "", 0, fmt.Errorf("unterminated json string")
}

func unmarshalString(raw []byte, dest *string) error {
	if len(raw) < 2 || raw[0] != '"' || raw[len(raw)-1] != '"' {
		return fmt.Errorf("invalid json string")
	}
	var buf bytes.Buffer
	buf.Grow(len(raw))
	for i := 1; i < len(raw)-1; i++ {
		if raw[i] != '\\' {
			buf.WriteByte(raw[i])
			continue
		}
		i++
		if i >= len(raw)-1 {
			return fmt.Errorf("invalid escape")
		}
		switch raw[i] {
		case '"', '\\', '/':
			buf.WriteByte(raw[i])
		case 'n':
			buf.WriteByte('\n')
		case 't':
			buf.WriteByte('\t')
		case 'r':
			buf.WriteByte('\r')
		default:
			buf.WriteByte(raw[i])
		}
	}
	*dest = buf.String()
	return nil
}

func skipValue(body []byte, i int) (int, error) {
	i = skipSpace(body, i)
	if i >= len(body) {
		return 0, fmt.Errorf("json value missing")
	}
	switch body[i] {
	case '"':
		return skipString(body, i)
	case '{':
		return skipBlock(body, i, '{', '}')
	case '[':
		return skipBlock(body, i, '[', ']')
	case 't':
		return skipLiteral(body, i, "true")
	case 'f':
		return skipLiteral(body, i, "false")
	case 'n':
		return skipLiteral(body, i, "null")
	default:
		for i < len(body) && (body[i] == '-' || body[i] == '+' || body[i] == '.' || body[i] == 'e' || body[i] == 'E' || (body[i] >= '0' && body[i] <= '9')) {
			i++
		}
		return i, nil
	}
}

func skipBlock(body []byte, i int, open, close byte) (int, error) {
	if i >= len(body) || body[i] != open {
		return 0, fmt.Errorf("invalid json block")
	}
	depth := 1
	i++
	for i < len(body) && depth > 0 {
		switch body[i] {
		case '"':
			end, err := skipString(body, i)
			if err != nil {
				return 0, err
			}
			i = end
		case open:
			depth++
			i++
		case close:
			depth--
			i++
		default:
			i++
		}
	}
	if depth != 0 {
		return 0, fmt.Errorf("unterminated json block")
	}
	return i, nil
}

func skipString(body []byte, i int) (int, error) {
	if i >= len(body) || body[i] != '"' {
		return 0, fmt.Errorf("invalid json string")
	}
	i++
	for i < len(body) {
		c := body[i]
		if c == '\\' {
			i += 2
			continue
		}
		if c == '"' {
			return i + 1, nil
		}
		i++
	}
	return 0, fmt.Errorf("unterminated json string")
}

func skipLiteral(body []byte, i int, lit string) (int, error) {
	if i+len(lit) > len(body) || string(body[i:i+len(lit)]) != lit {
		return 0, fmt.Errorf("invalid json literal")
	}
	return i + len(lit), nil
}

func skipSpace(body []byte, i int) int {
	for i < len(body) && unicode.IsSpace(rune(body[i])) {
		i++
	}
	return i
}

package migrate

import (
	"context"
	"errors"
	"testing"
)

func TestFromSourceRequiresDSN(t *testing.T) {
	cases := []struct {
		name string
		dsn  string
	}{
		{name: "empty", dsn: ""},
		{name: "whitespace", dsn: "   "},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := FromSource(context.Background(), c.dsn, nil, nil, Options{})
			if !errors.Is(err, ErrSourceNotConfigured) {
				t.Fatalf("err = %v, want %v", err, ErrSourceNotConfigured)
			}
		})
	}
}

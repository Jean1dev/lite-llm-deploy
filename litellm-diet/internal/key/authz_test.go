package key

import (
	"errors"
	"testing"
	"time"
)

func TestAuthenticateStatesThatBlockUse(t *testing.T) {
	now := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	budget := 10.0
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	cases := []struct {
		name string
		key  Key
		want error
	}{
		{name: "active key", key: Key{Hash: "abc"}},
		{name: "blocked key", key: Key{Hash: "abc", Blocked: true}, want: ErrKeyBlocked},
		{name: "expired key", key: Key{Hash: "abc", ExpiresAt: &past}, want: ErrKeyExpired},
		{name: "still valid", key: Key{Hash: "abc", ExpiresAt: &future}},
		{name: "budget exceeded", key: Key{Hash: "abc", Spend: 10, MaxBudget: &budget}, want: ErrBudgetExceeded},
		{name: "budget under limit", key: Key{Hash: "abc", Spend: 9.99, MaxBudget: &budget}},
		{name: "no budget configured", key: Key{Hash: "abc", Spend: 1000}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := Authenticate(c.key, now)
			if c.want == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
		})
	}
}

func TestAuthorizeModelEmptyWildcardAndDenied(t *testing.T) {
	cases := []struct {
		name   string
		models []string
		req    string
		want   error
	}{
		{name: "empty list allows all", models: nil, req: "anthropic/claude-haiku-4-5"},
		{name: "explicit empty list", models: []string{}, req: "openai/gpt-4o"},
		{name: "matching wildcard", models: []string{"openai/*"}, req: "openai/gpt-4o-mini"},
		{name: "explicit model", models: []string{"openai/gpt-4o"}, req: "openai/gpt-4o"},
		{name: "model outside list", models: []string{"openai/*"}, req: "anthropic/claude-haiku-4-5", want: ErrModelNotAllowed},
		{name: "other provider wildcard", models: []string{"gemini/*"}, req: "openai/gpt-4o", want: ErrModelNotAllowed},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := AuthorizeModel(Key{Models: c.models}, c.req)
			if c.want == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if !errors.Is(err, c.want) {
				t.Fatalf("err = %v, want %v", err, c.want)
			}
		})
	}
}

func TestMasterRejectsVirtualKey(t *testing.T) {
	if !Master("sk-dev-master", "sk-dev-master") {
		t.Fatal("master key rejected")
	}
	if Master("sk-virtual", "sk-dev-master") {
		t.Fatal("virtual key accepted as master")
	}
	if Master("", "sk-dev-master") {
		t.Fatal("empty key accepted")
	}
}

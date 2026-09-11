package provider

import (
	"fmt"
	"strings"
)

var ErrUnknownPrefix = fmt.Errorf("unknown provider prefix")

type ID string

const (
	OpenAI    ID = "openai"
	Anthropic ID = "anthropic"
	Gemini    ID = "gemini"
)

var Supported = []ID{OpenAI, Anthropic, Gemini}

var defaultBaseURLs = map[ID]string{
	OpenAI:    "https://api.openai.com/v1",
	Anthropic: "https://api.anthropic.com/v1",
	Gemini:    "https://generativelanguage.googleapis.com/v1beta",
}

func DefaultBaseURL(id ID) (string, bool) {
	u, ok := defaultBaseURLs[id]
	return u, ok
}

func IsSupported(id ID) bool {
	_, ok := defaultBaseURLs[id]
	return ok
}

func Split(name string) (ID, string, error) {
	idText, model, ok := strings.Cut(name, "/")
	if !ok || idText == "" || model == "" {
		return "", "", fmt.Errorf("%q: %w", name, ErrUnknownPrefix)
	}
	id := ID(idText)
	if !IsSupported(id) {
		return "", "", fmt.Errorf("%q: %w", name, ErrUnknownPrefix)
	}
	return id, model, nil
}

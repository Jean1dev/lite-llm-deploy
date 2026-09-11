package httpapi

import (
	"net/http"
	"strings"

	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/key"
)

func bearer(r *http.Request) string {
	v := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(v, prefix) {
		return strings.TrimSpace(v)
	}
	return strings.TrimSpace(strings.TrimPrefix(v, prefix))
}

func (s *Server) authenticate(r *http.Request) (key.Key, error) {
	plain := bearer(r)
	if plain == "" {
		return key.Key{}, key.ErrKeyNotFound
	}
	hash := key.HashToken(plain)
	k, ok := s.keys.Lookup(hash)
	if !ok {
		return key.Key{}, key.ErrKeyNotFound
	}
	if err := key.Authenticate(k, s.now()); err != nil {
		return key.Key{}, err
	}
	return k, nil
}

func (s *Server) requireMaster(w http.ResponseWriter, r *http.Request) bool {
	if key.Master(bearer(r), s.master) {
		return true
	}
	writeError(w, http.StatusUnauthorized, "authentication_error", "401", "invalid master key")
	return false
}

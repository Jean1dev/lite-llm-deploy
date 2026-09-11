package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/key"
	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/provider"
)

type errorEnvelope struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Param   any    `json:"param"`
		Code    string `json:"code"`
	} `json:"error"`
}

func writeError(w http.ResponseWriter, status int, typ, code, message string) {
	var env errorEnvelope
	env.Error.Message = message
	env.Error.Type = typ
	env.Error.Code = code
	body, err := json.Marshal(env)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"internal failure","type":"internal_error","param":null,"code":"internal_error"}}`))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func authorizationError(err error) (int, string, string, string) {
	switch {
	case errors.Is(err, key.ErrKeyNotFound), errors.Is(err, key.ErrKeyBlocked), errors.Is(err, key.ErrKeyExpired):
		return http.StatusUnauthorized, "authentication_error", "401", err.Error()
	case errors.Is(err, key.ErrBudgetExceeded):
		return http.StatusBadRequest, "budget_exceeded", "400", err.Error()
	case errors.Is(err, key.ErrModelNotAllowed):
		return http.StatusForbidden, "invalid_request_error", "403", err.Error()
	case errors.Is(err, provider.ErrUnknownPrefix):
		return http.StatusBadRequest, "invalid_request_error", "400", err.Error()
	default:
		return http.StatusInternalServerError, "internal_error", "500", "internal failure"
	}
}

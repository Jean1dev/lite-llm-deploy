package httpapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/key"
	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/storage"
)

type generateBody struct {
	KeyAlias       string   `json:"key_alias"`
	Models         []string `json:"models"`
	MaxBudget      *float64 `json:"max_budget"`
	BudgetDuration string   `json:"budget_duration"`
	Duration       string   `json:"duration"`
}

type keyBody struct {
	Key     string   `json:"key"`
	KeyHash string   `json:"key_hash"`
	Alias   string   `json:"key_alias"`
	Models  []string `json:"models"`
}

func (s *Server) generateKey(w http.ResponseWriter, r *http.Request) {
	if !s.requireMaster(w, r) {
		return
	}
	var req generateBody
	if err := decodeBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "400", "invalid body")
		return
	}
	plain, hash, err := key.Generate()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "500", "failed to generate key")
		return
	}
	now := s.now()
	k := key.Key{
		Hash:           hash,
		Alias:          req.KeyAlias,
		Models:         req.Models,
		MaxBudget:      req.MaxBudget,
		BudgetDuration: req.BudgetDuration,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if req.BudgetDuration != "" {
		reset, err := key.NextReset(now, req.BudgetDuration)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request_error", "400", err.Error())
			return
		}
		k.BudgetResetAt = &reset
	}
	if req.Duration != "" {
		d, err := key.ParseDuration(req.Duration)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request_error", "400", err.Error())
			return
		}
		exp := now.Add(d)
		k.ExpiresAt = &exp
	}
	if err := s.repo.Insert(r.Context(), k); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "500", "failed to persist key")
		return
	}
	s.keys.Replace(k)
	writeJSON(w, http.StatusOK, keyResponse(k, plain))
}

func (s *Server) updateKey(w http.ResponseWriter, r *http.Request) {
	if !s.requireMaster(w, r) {
		return
	}
	var patch struct {
		Key            string   `json:"key"`
		KeyHash        string   `json:"key_hash"`
		KeyAlias       *string  `json:"key_alias"`
		Models         []string `json:"models"`
		MaxBudget      *float64 `json:"max_budget"`
		BudgetDuration *string  `json:"budget_duration"`
	}
	if err := decodeBody(r, &patch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "400", "invalid body")
		return
	}
	k, ok := s.resolveKey(w, patch.Key, patch.KeyHash)
	if !ok {
		return
	}
	if patch.KeyAlias != nil {
		k.Alias = *patch.KeyAlias
	}
	if patch.Models != nil {
		k.Models = patch.Models
	}
	if patch.MaxBudget != nil {
		k.MaxBudget = patch.MaxBudget
	}
	if patch.BudgetDuration != nil {
		k.BudgetDuration = *patch.BudgetDuration
		if k.BudgetDuration != "" {
			reset, err := key.NextReset(s.now(), k.BudgetDuration)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid_request_error", "400", err.Error())
				return
			}
			k.BudgetResetAt = &reset
		}
	}
	k.UpdatedAt = s.now()
	if err := s.repo.Update(r.Context(), k); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "500", "failed to update key")
		return
	}
	s.keys.Replace(k)
	writeJSON(w, http.StatusOK, keyResponse(k, ""))
}

func (s *Server) deleteKey(w http.ResponseWriter, r *http.Request) {
	if !s.requireMaster(w, r) {
		return
	}
	k, ok := s.keyFromBody(w, r)
	if !ok {
		return
	}
	if err := s.repo.Delete(r.Context(), k.Hash); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "500", "failed to delete key")
		return
	}
	s.keys.Remove(k.Hash)
	writeJSON(w, http.StatusOK, map[string]any{"deleted": true, "key_hash": k.Hash})
}

func (s *Server) blockKey(w http.ResponseWriter, r *http.Request) {
	s.setBlocked(w, r, true)
}

func (s *Server) unblockKey(w http.ResponseWriter, r *http.Request) {
	s.setBlocked(w, r, false)
}

func (s *Server) setBlocked(w http.ResponseWriter, r *http.Request, blocked bool) {
	if !s.requireMaster(w, r) {
		return
	}
	k, ok := s.keyFromBody(w, r)
	if !ok {
		return
	}
	k.Blocked = blocked
	k.UpdatedAt = s.now()
	if err := s.repo.Update(r.Context(), k); err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "500", "failed to update key")
		return
	}
	s.keys.Replace(k)
	writeJSON(w, http.StatusOK, keyResponse(k, ""))
}

func (s *Server) keyInfo(w http.ResponseWriter, r *http.Request) {
	if !s.requireMaster(w, r) {
		return
	}
	hash := r.URL.Query().Get("key_hash")
	if hash == "" {
		if plain := r.URL.Query().Get("key"); plain != "" {
			hash = key.HashToken(plain)
		}
	}
	if hash == "" {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "400", "provide key or key_hash")
		return
	}
	k, err := s.repo.Get(r.Context(), hash)
	if errors.Is(err, storage.ErrKeyNotFound) {
		writeError(w, http.StatusNotFound, "not_found", "404", "key not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "500", "failed to look up key")
		return
	}
	if mem, ok := s.keys.Lookup(hash); ok {
		k.Spend = mem.Spend
		k.LastActive = mem.LastActive
		k.BudgetResetAt = mem.BudgetResetAt
	}
	writeJSON(w, http.StatusOK, map[string]any{"info": keyResponse(k, "")})
}

func (s *Server) listKeys(w http.ResponseWriter, r *http.Request) {
	if !s.requireMaster(w, r) {
		return
	}
	list := s.keys.All()
	items := make([]map[string]any, 0, len(list))
	for _, k := range list {
		items = append(items, keyResponse(k, ""))
	}
	writeJSON(w, http.StatusOK, map[string]any{"keys": items})
}

func (s *Server) keyFromBody(w http.ResponseWriter, r *http.Request) (key.Key, bool) {
	var req keyBody
	if err := decodeBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "400", "invalid body")
		return key.Key{}, false
	}
	return s.resolveKey(w, req.Key, req.KeyHash)
}

func (s *Server) resolveKey(w http.ResponseWriter, plain, hash string) (key.Key, bool) {
	if hash == "" && plain != "" {
		hash = key.HashToken(plain)
	}
	if hash == "" {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "400", "provide key or key_hash")
		return key.Key{}, false
	}
	k, ok := s.keys.Lookup(hash)
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "404", "key not found")
		return key.Key{}, false
	}
	return k, true
}

func keyResponse(k key.Key, plain string) map[string]any {
	out := map[string]any{
		"key_hash":        k.Hash,
		"key_alias":       k.Alias,
		"models":          k.Models,
		"spend":           k.Spend,
		"max_budget":      k.MaxBudget,
		"budget_duration": k.BudgetDuration,
		"budget_reset_at": k.BudgetResetAt,
		"last_active":     k.LastActive,
		"expires":         k.ExpiresAt,
		"blocked":         k.Blocked,
		"created_at":      k.CreatedAt,
		"updated_at":      k.UpdatedAt,
	}
	if plain != "" {
		out["key"] = plain
	}
	return out
}

func decodeBody(r *http.Request, dest any) error {
	defer func() { _ = r.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return err
	}
	if len(body) == 0 {
		return nil
	}
	return json.Unmarshal(body, dest)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/provider"
)

func (s *Server) prepareCatalog() error {
	ids := make([]provider.ID, 0, len(s.providers))
	for _, id := range provider.Supported {
		if _, ok := s.providers[id]; ok {
			ids = append(ids, id)
		}
	}
	entries := s.catalog.Expand(ids)

	list := struct {
		Object string           `json:"object"`
		Data   []map[string]any `json:"data"`
	}{Object: "list", Data: make([]map[string]any, 0, len(entries))}
	info := struct {
		Data []map[string]any `json:"data"`
	}{Data: make([]map[string]any, 0, len(entries))}

	now := time.Now().Unix()
	for _, e := range entries {
		list.Data = append(list.Data, map[string]any{
			"id":       e.Name(),
			"object":   "model",
			"created":  now,
			"owned_by": string(e.Provider),
		})
		info.Data = append(info.Data, map[string]any{
			"model_name": e.Name(),
			"model_info": map[string]any{
				"key":                   e.Name(),
				"max_input_tokens":      e.MaxInputTokens,
				"max_output_tokens":     e.MaxOutputTokens,
				"input_cost_per_token":  e.InputPrice,
				"output_cost_per_token": e.OutputPrice,
				"litellm_provider":      string(e.Provider),
				"mode":                  "chat",
			},
		})
	}

	var err error
	s.modelList, err = json.Marshal(list)
	if err != nil {
		return err
	}
	s.modelInfo, err = json.Marshal(info)
	return err
}

func (s *Server) listModels(w http.ResponseWriter, r *http.Request) {
	if _, err := s.authenticate(r); err != nil {
		status, typ, code, msg := authorizationError(err)
		writeError(w, status, typ, code, msg)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(s.modelList)
}

func (s *Server) modelInfoHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := s.authenticate(r); err != nil {
		status, typ, code, msg := authorizationError(err)
		writeError(w, status, typ, code, msg)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(s.modelInfo)
}

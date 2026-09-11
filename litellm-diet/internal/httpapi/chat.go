package httpapi

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/anthropic"
	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/catalog"
	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/config"
	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/gemini"
	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/jsonutil"
	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/key"
	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/provider"
)

const bodyLimit = 16 << 20

func (s *Server) chatCompletions(w http.ResponseWriter, r *http.Request) {
	k, err := s.authenticate(r)
	if err != nil {
		status, typ, code, msg := authorizationError(err)
		writeError(w, status, typ, code, msg)
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, bodyLimit))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "400", "invalid body")
		return
	}

	model, err := jsonutil.ExtractModel(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "400", "model field missing")
		return
	}
	if err := key.AuthorizeModel(k, model); err != nil {
		status, typ, code, msg := authorizationError(err)
		writeError(w, status, typ, code, msg)
		return
	}

	id, providerModel, err := provider.Split(model)
	if err != nil {
		status, typ, code, msg := authorizationError(err)
		writeError(w, status, typ, code, msg)
		return
	}
	cfg, ok := s.providers[id]
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "400", "provider not configured")
		return
	}

	streaming, _ := jsonutil.ExtractBool(body, "stream")
	includeUsage := clientWantsUsage(body)
	callID := newCallID()
	entry, _ := s.catalog.Entry(model)

	switch id {
	case provider.OpenAI:
		s.forwardOpenAI(w, r.Context(), k, cfg, body, model, providerModel, streaming, includeUsage, callID, entry)
	case provider.Anthropic:
		s.forwardAnthropic(w, r.Context(), k, cfg, body, model, providerModel, streaming, includeUsage, callID, entry)
	case provider.Gemini:
		s.forwardGemini(w, r.Context(), k, cfg, body, model, providerModel, streaming, includeUsage, callID, entry)
	}
}

func clientWantsUsage(body []byte) bool {
	obj, ok := jsonutil.ExtractObject(body, "stream_options")
	if !ok {
		return false
	}
	raw, ok := obj["include_usage"]
	if !ok {
		return false
	}
	var v bool
	if err := json.Unmarshal(raw, &v); err != nil {
		return false
	}
	return v
}

func (s *Server) forwardOpenAI(w http.ResponseWriter, ctx context.Context, k key.Key, cfg config.Provider, body []byte, model, providerModel string, streaming, includeUsage bool, callID string, entry catalog.Entry) {
	sent := body
	var err error
	sent, err = jsonutil.ReplaceStringField(sent, "model", providerModel)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "400", "invalid body")
		return
	}
	if streaming && !includeUsage {
		sent, err = ensureProviderUsage(sent)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request_error", "400", "invalid body")
			return
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(cfg.BaseURL, "/")+"/chat/completions", bytes.NewReader(sent))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "500", "internal failure")
		return
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Credential)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "api_error", "502", err.Error())
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		s.forwardProviderError(w, resp)
		return
	}

	if streaming {
		s.forwardStream(w, resp, k, model, callID, entry.Name(), includeUsage, func(chunk []byte) []byte {
			out, err := jsonutil.ReplaceStringField(chunk, "model", model)
			if err != nil {
				return chunk
			}
			return out
		})
		return
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		writeError(w, http.StatusBadGateway, "api_error", "502", "unreadable provider response")
		return
	}
	respBody, err = jsonutil.ReplaceStringField(respBody, "model", model)
	if err != nil {
		writeError(w, http.StatusBadGateway, "api_error", "502", "invalid provider response")
		return
	}
	s.writeProviderJSON(w, resp, k, model, callID, entry, respBody)
}

func ensureProviderUsage(body []byte) ([]byte, error) {
	obj, err := jsonutil.RemoveFields(body)
	if err != nil {
		return nil, err
	}
	_ = obj
	inner := []byte(`{"include_usage":true}`)
	decoded, err := decodeMap(body)
	if err != nil {
		return nil, err
	}
	decoded["stream_options"] = json.RawMessage(inner)
	return json.Marshal(decoded)
}

func decodeMap(body []byte) (map[string]json.RawMessage, error) {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(body, &obj); err != nil {
		return nil, err
	}
	return obj, nil
}

func (s *Server) forwardAnthropic(w http.ResponseWriter, ctx context.Context, k key.Key, cfg config.Provider, body []byte, model, providerModel string, streaming, includeUsage bool, callID string, entry catalog.Entry) {
	sent, err := anthropic.ConvertRequest(body, providerModel)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "400", err.Error())
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(cfg.BaseURL, "/")+"/messages", bytes.NewReader(sent))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "500", "internal failure")
		return
	}
	req.Header.Set("x-api-key", cfg.Credential)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "api_error", "502", err.Error())
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		s.forwardProviderError(w, resp)
		return
	}
	if streaming {
		s.translatedStream(w, resp, k, model, callID, entry, includeUsage, func(r io.Reader, emit func([]byte) error) (int, int, error) {
			return anthropic.ConvertStream(r, model, includeUsage, emit)
		})
		return
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		writeError(w, http.StatusBadGateway, "api_error", "502", "unreadable provider response")
		return
	}
	translated, err := anthropic.ConvertResponse(respBody, model)
	if err != nil {
		writeError(w, http.StatusBadGateway, "api_error", "502", err.Error())
		return
	}
	s.writeProviderJSON(w, resp, k, model, callID, entry, translated)
}

func (s *Server) forwardGemini(w http.ResponseWriter, ctx context.Context, k key.Key, cfg config.Provider, body []byte, model, providerModel string, streaming, includeUsage bool, callID string, entry catalog.Entry) {
	sent, err := gemini.ConvertRequest(body)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "400", err.Error())
		return
	}
	method := "generateContent"
	if streaming {
		method = "streamGenerateContent"
	}
	url := fmt.Sprintf("%s/models/%s:%s?key=%s", strings.TrimRight(cfg.BaseURL, "/"), providerModel, method, cfg.Credential)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(sent))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "500", "internal failure")
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "api_error", "502", err.Error())
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		s.forwardProviderError(w, resp)
		return
	}
	if streaming {
		s.translatedStream(w, resp, k, model, callID, entry, includeUsage, func(r io.Reader, emit func([]byte) error) (int, int, error) {
			return gemini.ConvertStream(r, model, includeUsage, emit)
		})
		return
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		writeError(w, http.StatusBadGateway, "api_error", "502", "unreadable provider response")
		return
	}
	translated, err := gemini.ConvertResponse(respBody, model)
	if err != nil {
		writeError(w, http.StatusBadGateway, "api_error", "502", err.Error())
		return
	}
	s.writeProviderJSON(w, resp, k, model, callID, entry, translated)
}

func (s *Server) writeProviderJSON(w http.ResponseWriter, resp *http.Response, k key.Key, model, callID string, entry catalog.Entry, body []byte) {
	usage, _ := extractUsage(body)
	cost := s.recordCost(k, model, usage)
	mirrorProviderHeaders(w.Header(), resp.Header)
	writeDiagnosticHeaders(w, callID, model, entry.Name(), cost, k.Spend+cost, false)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (s *Server) translatedStream(w http.ResponseWriter, resp *http.Response, k key.Key, model, callID string, entry catalog.Entry, includeUsage bool, convert func(io.Reader, func([]byte) error) (int, int, error)) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal_error", "500", "streaming unavailable")
		return
	}
	mirrorProviderHeaders(w.Header(), resp.Header)
	writeDiagnosticHeaders(w, callID, model, entry.Name(), 0, k.Spend, true)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	prompt, completion, err := convert(resp.Body, func(chunk []byte) error {
		if _, err := w.Write(anthropic.WrapSSE(chunk)); err != nil {
			return err
		}
		flusher.Flush()
		return nil
	})
	if err != nil && !errors.Is(err, io.EOF) {
		return
	}
	s.recordCost(k, model, responseUsage{PromptTokens: prompt, CompletionTokens: completion})
	_, _ = w.Write([]byte("data: [DONE]\n\n"))
	flusher.Flush()
}

func (s *Server) forwardStream(w http.ResponseWriter, resp *http.Response, k key.Key, model, callID, modelID string, includeUsage bool, rewrite func([]byte) []byte) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "internal_error", "500", "streaming unavailable")
		return
	}
	mirrorProviderHeaders(w.Header(), resp.Header)
	writeDiagnosticHeaders(w, callID, model, modelID, 0, k.Spend, true)
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	sc := bufio.NewScanner(resp.Body)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var usage responseUsage
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			continue
		}
		chunk := rewrite([]byte(payload))
		if u, ok := extractUsage(chunk); ok {
			usage = u
			if !includeUsage {
				continue
			}
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", chunk); err != nil {
			return
		}
		flusher.Flush()
	}
	s.recordCost(k, model, usage)
	_, _ = w.Write([]byte("data: [DONE]\n\n"))
	flusher.Flush()
}

func (s *Server) forwardProviderError(w http.ResponseWriter, resp *http.Response) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		writeError(w, resp.StatusCode, "api_error", fmt.Sprintf("%d", resp.StatusCode), "provider error")
		return
	}
	var env errorEnvelope
	if err := json.Unmarshal(body, &env); err == nil && env.Error.Message != "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		out, err := json.Marshal(env)
		if err != nil {
			writeError(w, resp.StatusCode, "api_error", fmt.Sprintf("%d", resp.StatusCode), env.Error.Message)
			return
		}
		_, _ = w.Write(out)
		return
	}
	writeError(w, resp.StatusCode, "api_error", fmt.Sprintf("%d", resp.StatusCode), strings.TrimSpace(string(body)))
}

func (s *Server) recordCost(k key.Key, model string, usage responseUsage) float64 {
	value, err := s.catalog.Cost(model, catalog.Usage{PromptTokens: usage.PromptTokens, CompletionTokens: usage.CompletionTokens})
	if err != nil {
		s.log.Warn("cost not computable", "model", model, "reason", err.Error())
		s.aggregator.Record(k.Hash, 0, s.now())
		return 0
	}
	s.aggregator.Record(k.Hash, value, s.now())
	return value
}

func newCallID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("call-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

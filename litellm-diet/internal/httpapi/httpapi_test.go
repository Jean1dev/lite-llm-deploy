package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/catalog"
	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/config"
	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/key"
	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/memory"
	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/migrate"
	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/provider"
	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/storage"
)

type fakeRepo struct {
	keys []key.Key
}

func (r *fakeRepo) List(_ context.Context) ([]key.Key, error) { return r.keys, nil }

type nullWriter struct{}

func (nullWriter) AddSpend(context.Context, []storage.SpendDelta) error { return nil }

func testServer(t *testing.T, openaiURL string) (*Server, string) {
	t.Helper()
	cat, err := catalog.LoadEmbedded()
	if err != nil {
		t.Fatalf("catalog: %v", err)
	}
	plain, hash, err := key.Generate()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	now := time.Now().UTC()
	k := key.Key{Hash: hash, CreatedAt: now, UpdatedAt: now}
	reader := &fakeRepo{keys: []key.Key{k}}
	m := memory.NewMap(reader)
	if err := m.Load(context.Background()); err != nil {
		t.Fatalf("load map: %v", err)
	}
	ag := memory.NewAggregator(nullWriter{}, m)
	srv, err := NewServer(Dependencies{
		Port:      0,
		MasterKey: "sk-master",
		Providers: map[provider.ID]config.Provider{
			provider.OpenAI:    {ID: provider.OpenAI, Credential: "sk-openai", BaseURL: openaiURL},
			provider.Anthropic: {ID: provider.Anthropic, Credential: "sk-ant", BaseURL: openaiURL},
			provider.Gemini:    {ID: provider.Gemini, Credential: "sk-gem", BaseURL: openaiURL},
		},
		Catalog:    cat,
		Map:        m,
		Aggregator: ag,
		Ready:      func(context.Context) error { return nil },
		Log:        slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if err != nil {
		t.Fatalf("server: %v", err)
	}
	return srv, plain
}

func TestVersionedAndUnversionedPaths(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("x-ratelimit-remaining", "10")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-up","object":"chat.completion","created":1,"model":"gpt-4o","choices":[{"index":0,"message":{"role":"assistant","content":"ok","extra":"keep"},"finish_reason":"stop"}],"usage":{"prompt_tokens":2,"completion_tokens":1,"total_tokens":3}}`))
	}))
	t.Cleanup(upstream.Close)

	srv, token := testServer(t, upstream.URL)
	body := `{"model":"openai/gpt-4o","messages":[{"role":"user","content":"hi"}]}`

	req1 := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	req1.Header.Set("Authorization", "Bearer "+token)
	rec1 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec1, req1)

	req2 := httptest.NewRequest(http.MethodPost, "/chat/completions", strings.NewReader(body))
	req2.Header.Set("Authorization", "Bearer "+token)
	rec2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec2, req2)

	if rec1.Code != http.StatusOK || rec2.Code != http.StatusOK {
		t.Fatalf("status %d / %d body=%s", rec1.Code, rec2.Code, rec1.Body.String())
	}
	var a, b map[string]any
	if err := json.Unmarshal(rec1.Body.Bytes(), &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	if a["model"] != "openai/gpt-4o" || b["model"] != a["model"] {
		t.Errorf("divergent models %v %v", a["model"], b["model"])
	}
	if rec1.Header().Get("x-litellm-call-id") == "" {
		t.Error("diagnostic header missing")
	}
	if rec1.Header().Get("llm_provider-X-Ratelimit-Remaining") == "" && rec1.Header().Get("llm_provider-x-ratelimit-remaining") == "" {
		t.Errorf("provider header not mirrored: %v", rec1.Header())
	}
}

func TestUnknownPrefix(t *testing.T) {
	srv, token := testServer(t, "http://127.0.0.1:9")
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"mistral/x","messages":[]}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"error"`)) {
		t.Errorf("envelope missing: %s", rec.Body.String())
	}
}

func TestAuthenticationTable(t *testing.T) {
	srv, token := testServer(t, "http://127.0.0.1:9")
	cases := []struct {
		name   string
		auth   string
		status int
	}{
		{name: "missing key", auth: "", status: http.StatusUnauthorized},
		{name: "unknown key", auth: "Bearer sk-missing", status: http.StatusUnauthorized},
		{name: "valid key invalid prefix", auth: "Bearer " + token, status: http.StatusBadRequest},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"x","messages":[]}`))
			if c.auth != "" {
				req.Header.Set("Authorization", c.auth)
			}
			rec := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rec, req)
			if rec.Code != c.status {
				t.Fatalf("status = %d, want %d body=%s", rec.Code, c.status, rec.Body.String())
			}
		})
	}
}

func TestMasterRejectsVirtualKey(t *testing.T) {
	srv, token := testServer(t, "http://127.0.0.1:9")
	req := httptest.NewRequest(http.MethodPost, "/key/generate", strings.NewReader(`{"key_alias":"x"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestModelListingIsPreserialized(t *testing.T) {
	srv, token := testServer(t, "http://127.0.0.1:9")
	get := func(path string) []byte {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status %d %s", path, rec.Code, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	a := get("/v1/models")
	b := get("/models")
	if !bytes.Equal(a, b) {
		t.Fatal("models paths diverge")
	}
	c := get("/v1/models")
	if !bytes.Equal(a, c) {
		t.Fatal("models body changed between calls")
	}
	info := get("/v1/model/info")
	if !bytes.Contains(info, []byte("input_cost_per_token")) || !bytes.Contains(info, []byte("max_input_tokens")) {
		t.Fatalf("metadata missing: %s", info)
	}
}

func TestPreservesUnknownOpenAIField(t *testing.T) {
	var received []byte
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"x","object":"chat.completion","created":1,"model":"gpt-4o","choices":[{"index":0,"message":{"role":"assistant","content":"ok","custom":true},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2},"mystery":1}`))
	}))
	t.Cleanup(upstream.Close)
	srv, token := testServer(t, upstream.URL)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"openai/gpt-4o","messages":[{"role":"user","content":"hi"}],"foo_bar":123}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(received, []byte(`"foo_bar"`)) {
		t.Fatalf("unknown field did not reach provider: %s", received)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte(`"mystery"`)) {
		t.Fatalf("unknown response field lost: %s", rec.Body.String())
	}
}

func TestIncrementalStreaming(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fl, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("data: {\"id\":\"c\",\"object\":\"chat.completion.chunk\",\"model\":\"gpt-4o\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"a\"},\"finish_reason\":null}]}\n\n"))
		fl.Flush()
		time.Sleep(20 * time.Millisecond)
		_, _ = w.Write([]byte("data: {\"id\":\"c\",\"object\":\"chat.completion.chunk\",\"model\":\"gpt-4o\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":1,\"completion_tokens\":1,\"total_tokens\":2}}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		fl.Flush()
	}))
	t.Cleanup(upstream.Close)
	srv, token := testServer(t, upstream.URL)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"openai/gpt-4o","messages":[{"role":"user","content":"hi"}],"stream":true}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, "data: ") || !strings.Contains(body, "[DONE]") {
		t.Fatalf("invalid stream: %s", body)
	}
	if strings.Count(body, `"usage"`) != 0 {
		t.Fatalf("usage leaked without request: %s", body)
	}
}

func TestReadinessReflectsDatabase(t *testing.T) {
	srv, _ := testServer(t, "http://127.0.0.1:9")
	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("ready = %d", rec.Code)
	}
	srv.ready = func(context.Context) error { return io.EOF }
	rec = httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("ready unavailable = %d", rec.Code)
	}
}

type fakeImporter struct {
	repo     *fakeRepo
	imported key.Key
	lastDry  bool
	err      error
}

func (f *fakeImporter) Import(_ context.Context, dryRun bool) (migrate.Report, error) {
	f.lastDry = dryRun
	if f.err != nil {
		return migrate.Report{}, f.err
	}
	if !dryRun && f.repo != nil {
		f.repo.keys = append(f.repo.keys, f.imported)
	}
	return migrate.Report{Read: 1, Created: 1, DryRun: dryRun}, nil
}

func TestMigrateKeysRequiresMaster(t *testing.T) {
	srv, token := testServer(t, "http://127.0.0.1:9")
	cases := []struct {
		name   string
		auth   string
		status int
	}{
		{name: "missing", auth: "", status: http.StatusUnauthorized},
		{name: "virtual key", auth: "Bearer " + token, status: http.StatusUnauthorized},
		{name: "unknown", auth: "Bearer sk-missing", status: http.StatusUnauthorized},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/admin/migrate-keys", nil)
			if c.auth != "" {
				req.Header.Set("Authorization", c.auth)
			}
			rec := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rec, req)
			if rec.Code != c.status {
				t.Fatalf("status = %d, want %d body=%s", rec.Code, c.status, rec.Body.String())
			}
		})
	}
}

func TestMigrateKeysRejectsMissingSource(t *testing.T) {
	srv, _ := testServer(t, "http://127.0.0.1:9")
	req := httptest.NewRequest(http.MethodPost, "/admin/migrate-keys", nil)
	req.Header.Set("Authorization", "Bearer sk-master")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503 body=%s", rec.Code, rec.Body.String())
	}
}

func TestMigrateKeysImportsAndReloads(t *testing.T) {
	srv, _ := testServer(t, "http://127.0.0.1:9")
	_, hash, err := key.Generate()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	mapReader := &fakeRepo{}
	m := memory.NewMap(mapReader)
	if err := m.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	imp := &fakeImporter{
		repo:     mapReader,
		imported: key.Key{Hash: hash, Alias: "migrated", CreatedAt: now, UpdatedAt: now},
	}
	srv.keys = m
	srv.importer = imp
	srv.sourceDatabaseURL = "postgres://lite@db/railway"

	req := httptest.NewRequest(http.MethodPost, "/admin/migrate-keys", nil)
	req.Header.Set("Authorization", "Bearer sk-master")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if imp.lastDry {
		t.Fatal("expected write import")
	}
	if _, ok := srv.keys.Lookup(hash); !ok {
		t.Fatal("imported key not loaded")
	}
}

func TestMigrateKeysDryRunDoesNotReload(t *testing.T) {
	srv, _ := testServer(t, "http://127.0.0.1:9")
	_, hash, err := key.Generate()
	if err != nil {
		t.Fatal(err)
	}
	mapReader := &fakeRepo{}
	m := memory.NewMap(mapReader)
	if err := m.Load(context.Background()); err != nil {
		t.Fatal(err)
	}
	srv.keys = m
	srv.sourceDatabaseURL = "postgres://lite@db/railway"
	srv.importer = &fakeImporter{
		repo:     mapReader,
		imported: key.Key{Hash: hash},
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/migrate-keys?dry_run=true", nil)
	req.Header.Set("Authorization", "Bearer sk-master")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if _, ok := srv.keys.Lookup(hash); ok {
		t.Fatal("dry-run must not load keys")
	}
}

func TestMigrateKeysRejectsConcurrentRun(t *testing.T) {
	srv, _ := testServer(t, "http://127.0.0.1:9")
	srv.importer = &fakeImporter{}
	srv.sourceDatabaseURL = "postgres://lite@db/railway"
	srv.migrateMu.Lock()
	t.Cleanup(srv.migrateMu.Unlock)

	req := httptest.NewRequest(http.MethodPost, "/admin/migrate-keys", nil)
	req.Header.Set("Authorization", "Bearer sk-master")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 body=%s", rec.Code, rec.Body.String())
	}
}

func TestMigrateKeysReportsImportFailure(t *testing.T) {
	srv, _ := testServer(t, "http://127.0.0.1:9")
	srv.importer = &fakeImporter{err: io.EOF}
	srv.sourceDatabaseURL = "postgres://lite@db/railway"
	req := httptest.NewRequest(http.MethodPost, "/admin/migrate-keys", nil)
	req.Header.Set("Authorization", "Bearer sk-master")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502 body=%s", rec.Code, rec.Body.String())
	}
}

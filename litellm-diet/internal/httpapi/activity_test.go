package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/storage"
)

func TestBuildActivityGroupsByDayWithBreakdowns(t *testing.T) {
	d1 := time.Date(2026, 9, 29, 0, 0, 0, 0, time.UTC)
	d2 := time.Date(2026, 9, 30, 0, 0, 0, 0, time.UTC)
	rows := []storage.UsageRow{
		{Date: d1, KeyHash: "k1", Model: "openai/gpt-4o", Provider: "openai", PromptTokens: 10, CompletionTokens: 5, Spend: 1, Requests: 2, Successes: 2},
		{Date: d2, KeyHash: "k1", Model: "openai/gpt-4o", Provider: "openai", PromptTokens: 4, CompletionTokens: 1, Spend: 0.5, Requests: 2, Successes: 1, Failures: 1},
		{Date: d2, KeyHash: "k2", Model: "anthropic/claude-haiku-4-5", Provider: "anthropic", PromptTokens: 6, Spend: 0.25, Requests: 1, Successes: 1},
	}
	got := buildActivity(rows, func(h string) string { return "alias-" + h })

	if len(got.Results) != 2 || got.Results[0].Date != "2026-09-29" || got.Results[1].Date != "2026-09-30" {
		t.Fatalf("results = %+v", got.Results)
	}
	day := got.Results[1]
	if day.Metrics.APIRequests != 3 || day.Metrics.FailedRequests != 1 || day.Metrics.TotalTokens != 11 || day.Metrics.Spend != 0.75 {
		t.Errorf("day metrics = %+v", day.Metrics)
	}
	if len(day.Breakdown.Models) != 2 || len(day.Breakdown.Providers) != 2 || len(day.Breakdown.APIKeys) != 2 {
		t.Errorf("breakdown = %+v", day.Breakdown)
	}
	if a := day.Breakdown.APIKeys["k2"]; a == nil || a.Metadata["key_alias"] != "alias-k2" || a.Metrics.PromptTokens != 6 {
		t.Errorf("api key breakdown = %+v", a)
	}
	m := got.Metadata
	if m.TotalSpend != 1.75 || m.TotalAPIRequests != 5 || m.TotalTokens != 26 || m.TotalFailedRequests != 1 || m.Page != 1 || m.TotalPages != 1 || m.HasMore {
		t.Errorf("metadata = %+v", m)
	}
}

func TestActivityRange(t *testing.T) {
	now := time.Date(2026, 9, 30, 15, 0, 0, 0, time.UTC)
	cases := []struct {
		name, start, end string
		from, to         string
		bad              bool
	}{
		{name: "default seven days", from: "2026-09-24", to: "2026-09-30"},
		{name: "thirty days", start: "2026-09-01", end: "2026-09-30", from: "2026-09-01", to: "2026-09-30"},
		{name: "end only", end: "2026-09-10", from: "2026-09-04", to: "2026-09-10"},
		{name: "invalid start", start: "30/09/2026", bad: true},
		{name: "start after end", start: "2026-09-30", end: "2026-09-01", bad: true},
		{name: "too wide", start: "2025-01-01", end: "2026-09-30", bad: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			from, to, msg := activityRange(c.start, c.end, now)
			if c.bad {
				if msg == "" {
					t.Fatal("expected error")
				}
				return
			}
			if msg != "" {
				t.Fatalf("unexpected error: %s", msg)
			}
			if from.Format(activityDateLayout) != c.from || to.Format(activityDateLayout) != c.to {
				t.Errorf("range = %s..%s, want %s..%s", from.Format(activityDateLayout), to.Format(activityDateLayout), c.from, c.to)
			}
		})
	}
}

func TestDailyActivityRequiresMaster(t *testing.T) {
	srv, token := testServer(t, "http://127.0.0.1:9")
	for _, auth := range []string{"", "Bearer " + token} {
		req := httptest.NewRequest(http.MethodGet, "/user/daily/activity", nil)
		if auth != "" {
			req.Header.Set("Authorization", auth)
		}
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("auth %q: status = %d", auth, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/user/daily/activity?start_date=bad", nil)
	req.Header.Set("Authorization", "Bearer sk-master")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("bad date status = %d", rec.Code)
	}
}

func TestDailyActivityReflectsChatTraffic(t *testing.T) {
	fail := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if fail {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"message":"boom","type":"server_error"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"x","object":"chat.completion","created":1,"model":"gpt-4o","choices":[],"usage":{"prompt_tokens":100,"completion_tokens":20,"total_tokens":120}}`))
	}))
	t.Cleanup(upstream.Close)

	srv, token := testServer(t, upstream.URL)
	chat := func() int {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"openai/gpt-4o","messages":[{"role":"user","content":"hi"}]}`))
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		return rec.Code
	}
	for i := 0; i < 2; i++ {
		if code := chat(); code != http.StatusOK {
			t.Fatalf("chat status = %d", code)
		}
	}
	fail = true
	if code := chat(); code != http.StatusInternalServerError {
		t.Fatalf("failing chat status = %d", code)
	}
	if err := srv.aggregator.Flush(context.Background()); err != nil {
		t.Fatalf("flush: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/user/daily/activity", nil)
	req.Header.Set("Authorization", "Bearer sk-master")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("activity status = %d body=%s", rec.Code, rec.Body.String())
	}
	var got activityResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Results) != 1 {
		t.Fatalf("results = %s", rec.Body.String())
	}
	m := got.Metadata
	if m.TotalAPIRequests != 3 || m.TotalSuccessfulRequests != 2 || m.TotalFailedRequests != 1 || m.TotalPromptTokens != 200 || m.TotalCompletionTokens != 40 {
		t.Errorf("metadata = %+v", m)
	}
	if m.TotalSpend <= 0 {
		t.Errorf("total spend = %g, want > 0", m.TotalSpend)
	}
	if p := got.Results[0].Breakdown.Providers["openai"]; p == nil || p.Metrics.APIRequests != 3 {
		t.Errorf("provider breakdown = %+v", got.Results[0].Breakdown.Providers)
	}
}

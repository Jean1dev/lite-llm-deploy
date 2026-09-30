package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/storage"
)

const (
	activityDateLayout  = "2006-01-02"
	activityDefaultDays = 7
	activityMaxDays     = 366
)

type usageReader interface {
	DailyUsage(ctx context.Context, from, to time.Time, keyHash, model string) ([]storage.UsageRow, error)
}

type spendMetrics struct {
	Spend                    float64 `json:"spend"`
	PromptTokens             int64   `json:"prompt_tokens"`
	CompletionTokens         int64   `json:"completion_tokens"`
	CacheReadInputTokens     int64   `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int64   `json:"cache_creation_input_tokens"`
	TotalTokens              int64   `json:"total_tokens"`
	SuccessfulRequests       int64   `json:"successful_requests"`
	FailedRequests           int64   `json:"failed_requests"`
	APIRequests              int64   `json:"api_requests"`
}

func (m *spendMetrics) add(u storage.UsageRow) {
	m.Spend += u.Spend
	m.PromptTokens += u.PromptTokens
	m.CompletionTokens += u.CompletionTokens
	m.TotalTokens += u.PromptTokens + u.CompletionTokens
	m.SuccessfulRequests += u.Successes
	m.FailedRequests += u.Failures
	m.APIRequests += u.Requests
}

type metricWithMetadata struct {
	Metrics  spendMetrics   `json:"metrics"`
	Metadata map[string]any `json:"metadata"`
}

type activityBreakdown struct {
	Models    map[string]*metricWithMetadata `json:"models"`
	Providers map[string]*metricWithMetadata `json:"providers"`
	APIKeys   map[string]*metricWithMetadata `json:"api_keys"`
}

type dailySpend struct {
	Date      string            `json:"date"`
	Metrics   spendMetrics      `json:"metrics"`
	Breakdown activityBreakdown `json:"breakdown"`
}

type activityMetadata struct {
	TotalSpend                    float64 `json:"total_spend"`
	TotalPromptTokens             int64   `json:"total_prompt_tokens"`
	TotalCompletionTokens         int64   `json:"total_completion_tokens"`
	TotalTokens                   int64   `json:"total_tokens"`
	TotalAPIRequests              int64   `json:"total_api_requests"`
	TotalSuccessfulRequests       int64   `json:"total_successful_requests"`
	TotalFailedRequests           int64   `json:"total_failed_requests"`
	TotalCacheReadInputTokens     int64   `json:"total_cache_read_input_tokens"`
	TotalCacheCreationInputTokens int64   `json:"total_cache_creation_input_tokens"`
	Page                          int     `json:"page"`
	TotalPages                    int     `json:"total_pages"`
	HasMore                       bool    `json:"has_more"`
}

type activityResponse struct {
	Results  []*dailySpend    `json:"results"`
	Metadata activityMetadata `json:"metadata"`
}

func (s *Server) dailyActivity(w http.ResponseWriter, r *http.Request) {
	if !s.requireMaster(w, r) {
		return
	}
	q := r.URL.Query()
	from, to, msg := activityRange(q.Get("start_date"), q.Get("end_date"), s.now())
	if msg != "" {
		writeError(w, http.StatusBadRequest, "invalid_request_error", "400", msg)
		return
	}
	if s.usage == nil {
		writeError(w, http.StatusServiceUnavailable, "internal_error", "503", "usage storage not configured")
		return
	}
	rows, err := s.usage.DailyUsage(r.Context(), from, to, q.Get("api_key"), q.Get("model"))
	if err != nil {
		s.log.Error("query daily usage", "err", err.Error())
		writeError(w, http.StatusInternalServerError, "internal_error", "500", "failed to read usage")
		return
	}
	writeJSON(w, http.StatusOK, buildActivity(rows, func(hash string) string {
		if k, ok := s.keys.Lookup(hash); ok {
			return k.Alias
		}
		return ""
	}))
}

func activityRange(start, end string, now time.Time) (time.Time, time.Time, string) {
	today := now.UTC().Truncate(24 * time.Hour)
	to := today
	if end != "" {
		t, err := time.Parse(activityDateLayout, end)
		if err != nil {
			return time.Time{}, time.Time{}, "end_date must be YYYY-MM-DD"
		}
		to = t
	}
	from := to.AddDate(0, 0, -(activityDefaultDays - 1))
	if start != "" {
		t, err := time.Parse(activityDateLayout, start)
		if err != nil {
			return time.Time{}, time.Time{}, "start_date must be YYYY-MM-DD"
		}
		from = t
	}
	if from.After(to) {
		return time.Time{}, time.Time{}, "start_date must not be after end_date"
	}
	if to.Sub(from) >= activityMaxDays*24*time.Hour {
		return time.Time{}, time.Time{}, "date range must not exceed 366 days"
	}
	return from, to, ""
}

func buildActivity(rows []storage.UsageRow, alias func(string) string) activityResponse {
	out := activityResponse{
		Results:  make([]*dailySpend, 0, 32),
		Metadata: activityMetadata{Page: 1, TotalPages: 1},
	}
	byDate := make(map[string]*dailySpend)
	for _, u := range rows {
		date := u.Date.UTC().Format(activityDateLayout)
		day, ok := byDate[date]
		if !ok {
			day = &dailySpend{
				Date: date,
				Breakdown: activityBreakdown{
					Models:    make(map[string]*metricWithMetadata),
					Providers: make(map[string]*metricWithMetadata),
					APIKeys:   make(map[string]*metricWithMetadata),
				},
			}
			byDate[date] = day
			out.Results = append(out.Results, day)
		}
		day.Metrics.add(u)
		entry(day.Breakdown.Models, u.Model, nil).Metrics.add(u)
		entry(day.Breakdown.Providers, u.Provider, nil).Metrics.add(u)
		entry(day.Breakdown.APIKeys, u.KeyHash, func() map[string]any {
			return map[string]any{"key_alias": alias(u.KeyHash), "team_id": nil}
		}).Metrics.add(u)

		m := &out.Metadata
		m.TotalSpend += u.Spend
		m.TotalPromptTokens += u.PromptTokens
		m.TotalCompletionTokens += u.CompletionTokens
		m.TotalTokens += u.PromptTokens + u.CompletionTokens
		m.TotalAPIRequests += u.Requests
		m.TotalSuccessfulRequests += u.Successes
		m.TotalFailedRequests += u.Failures
	}
	return out
}

func entry(m map[string]*metricWithMetadata, name string, metadata func() map[string]any) *metricWithMetadata {
	e, ok := m[name]
	if ok {
		return e
	}
	e = &metricWithMetadata{Metadata: map[string]any{}}
	if metadata != nil {
		e.Metadata = metadata()
	}
	m[name] = e
	return e
}

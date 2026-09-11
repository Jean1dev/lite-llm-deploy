package httpapi

import (
	"net/http"
	"strconv"
	"strings"
)

func writeDiagnosticHeaders(w http.ResponseWriter, callID, model, modelID string, cost, spend float64, streaming bool) {
	w.Header().Set("x-litellm-call-id", callID)
	w.Header().Set("x-litellm-model-group", model)
	w.Header().Set("x-litellm-model-name", model)
	w.Header().Set("x-litellm-model-id", modelID)
	if streaming {
		w.Header().Set("x-litellm-response-cost", "0")
	} else {
		w.Header().Set("x-litellm-response-cost", formatCost(cost))
	}
	w.Header().Set("x-litellm-key-spend", formatCost(spend))
}

func mirrorProviderHeaders(dst http.Header, src http.Header) {
	for name, values := range src {
		if omitProviderHeader(name) {
			continue
		}
		for _, v := range values {
			dst.Add("llm_provider-"+name, v)
		}
	}
}

func omitProviderHeader(name string) bool {
	n := strings.ToLower(name)
	switch n {
	case "content-length", "content-encoding", "transfer-encoding", "connection", "keep-alive":
		return true
	default:
		return false
	}
}

func formatCost(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

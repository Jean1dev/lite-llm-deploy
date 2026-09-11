package harness

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestCompareResponsesDetectsInjectedDivergence(t *testing.T) {
	a := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}}
	b := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}}
	same := CompareResponses(a, b, []byte(`{"id":"1","model":"x","created":1}`), []byte(`{"id":"2","model":"x","created":9}`))
	if len(same) != 0 {
		t.Fatalf("id/created should be ignored: %+v", same)
	}
	diffs := CompareResponses(a, b, []byte(`{"id":"1","model":"x"}`), []byte(`{"id":"1","model":"y"}`))
	if len(diffs) == 0 {
		t.Fatal("model divergence not detected")
	}
}

func TestCompareStreamsDetectsOrderAndContent(t *testing.T) {
	a := "data: {\"choices\":[{\"delta\":{\"content\":\"a\"}}]}\n\ndata: [DONE]\n"
	b := "data: {\"choices\":[{\"delta\":{\"content\":\"b\"}}]}\n\ndata: [DONE]\n"
	if diffs := CompareStreams(strings.NewReader(a), strings.NewReader(b)); len(diffs) == 0 {
		t.Fatal("content difference not detected")
	}
	c := "data: {\"choices\":[{\"delta\":{\"content\":\"a\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"b\"}}]}\n\ndata: [DONE]\n"
	d := "data: {\"choices\":[{\"delta\":{\"content\":\"b\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"a\"}}]}\n\ndata: [DONE]\n"
	if diffs := CompareStreams(strings.NewReader(c), strings.NewReader(d)); len(diffs) == 0 {
		t.Fatal("order difference not detected")
	}
	_ = io.Discard
}

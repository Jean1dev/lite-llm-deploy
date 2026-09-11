package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/harness"
)

func main() {
	old := flag.String("old", "", "LiteLLM URL")
	neu := flag.String("new", "", "litellm-diet URL")
	key := flag.String("key", "", "virtual key")
	body := flag.String("body", `{"model":"openai/gpt-4o-mini","messages":[{"role":"user","content":"ping"}]}`, "request body")
	stream := flag.Bool("stream", false, "compare SSE stream")
	flag.Parse()

	if err := run(*old, *neu, *key, []byte(*body), *stream); err != nil {
		fmt.Fprintf(os.Stderr, "harness: %v\n", err)
		os.Exit(1)
	}
}

func run(old, neu, key string, body []byte, stream bool) error {
	if old == "" || neu == "" || key == "" {
		return fmt.Errorf("provide -old, -new and -key")
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	ra, err := fire(client, old, key, body)
	if err != nil {
		return fmt.Errorf("old: %w", err)
	}
	defer func() { _ = ra.Body.Close() }()
	rb, err := fire(client, neu, key, body)
	if err != nil {
		return fmt.Errorf("new: %w", err)
	}
	defer func() { _ = rb.Body.Close() }()

	var diffs []harness.Diff
	if stream {
		diffs = harness.CompareStreams(ra.Body, rb.Body)
	} else {
		ca, err := io.ReadAll(ra.Body)
		if err != nil {
			return err
		}
		cb, err := io.ReadAll(rb.Body)
		if err != nil {
			return err
		}
		diffs = harness.CompareResponses(ra, rb, ca, cb)
	}
	if len(diffs) == 0 {
		fmt.Println("no differences")
		return nil
	}
	for _, d := range diffs {
		fmt.Printf("%s: %s <> %s\n", d.Field, d.A, d.B)
	}
	return fmt.Errorf("%d differences", len(diffs))
}

func fire(c *http.Client, base, key string, body []byte) (*http.Response, error) {
	url := strings.TrimRight(base, "/") + "/v1/chat/completions"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	return c.Do(req)
}

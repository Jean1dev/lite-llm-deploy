package harness

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

var AllowedFields = []string{"id", "created", "x-litellm-call-id", "system_fingerprint"}

type Diff struct {
	Field string `json:"field"`
	A     string `json:"a"`
	B     string `json:"b"`
}

func CompareResponses(a, b *http.Response, bodyA, bodyB []byte) []Diff {
	var diffs []Diff
	if a.StatusCode != b.StatusCode {
		diffs = append(diffs, Diff{Field: "status", A: fmt.Sprintf("%d", a.StatusCode), B: fmt.Sprintf("%d", b.StatusCode)})
	}
	diffs = append(diffs, compareJSON(bodyA, bodyB, "")...)
	for name, values := range a.Header {
		if skipHeader(name) {
			continue
		}
		other := b.Header.Values(name)
		if strings.Join(values, ",") != strings.Join(other, ",") {
			diffs = append(diffs, Diff{Field: "header:" + name, A: strings.Join(values, ","), B: strings.Join(other, ",")})
		}
	}
	return diffs
}

func CompareStreams(a, b io.Reader) []Diff {
	chunksA := readChunks(a)
	chunksB := readChunks(b)
	if len(chunksA) != len(chunksB) {
		return []Diff{{Field: "chunks", A: fmt.Sprintf("%d", len(chunksA)), B: fmt.Sprintf("%d", len(chunksB))}}
	}
	var diffs []Diff
	for i := range chunksA {
		if chunksA[i] == "[DONE]" || chunksB[i] == "[DONE]" {
			if chunksA[i] != chunksB[i] {
				diffs = append(diffs, Diff{Field: fmt.Sprintf("chunk[%d]", i), A: chunksA[i], B: chunksB[i]})
			}
			continue
		}
		diffs = append(diffs, compareJSON([]byte(chunksA[i]), []byte(chunksB[i]), fmt.Sprintf("chunk[%d].", i))...)
	}
	return diffs
}

func readChunks(r io.Reader) []string {
	sc := bufio.NewScanner(r)
	out := make([]string, 0, 16)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		out = append(out, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
	}
	return out
}

func compareJSON(a, b []byte, prefix string) []Diff {
	var oa, ob any
	if err := json.Unmarshal(a, &oa); err != nil {
		if !bytes.Equal(a, b) {
			return []Diff{{Field: prefix + "body", A: string(a), B: string(b)}}
		}
		return nil
	}
	if err := json.Unmarshal(b, &ob); err != nil {
		return []Diff{{Field: prefix + "body", A: string(a), B: string(b)}}
	}
	var diffs []Diff
	walk(oa, ob, prefix, &diffs)
	return diffs
}

func walk(a, b any, path string, diffs *[]Diff) {
	if allowed(path) {
		return
	}
	switch va := a.(type) {
	case map[string]any:
		vb, ok := b.(map[string]any)
		if !ok {
			*diffs = append(*diffs, Diff{Field: path, A: fmt.Sprint(a), B: fmt.Sprint(b)})
			return
		}
		for k, va2 := range va {
			walk(va2, vb[k], join(path, k), diffs)
		}
	case []any:
		vb, ok := b.([]any)
		if !ok || len(va) != len(vb) {
			*diffs = append(*diffs, Diff{Field: path, A: fmt.Sprint(a), B: fmt.Sprint(b)})
			return
		}
		for i := range va {
			walk(va[i], vb[i], fmt.Sprintf("%s[%d]", path, i), diffs)
		}
	default:
		if fmt.Sprint(a) != fmt.Sprint(b) {
			*diffs = append(*diffs, Diff{Field: path, A: fmt.Sprint(a), B: fmt.Sprint(b)})
		}
	}
}

func join(prefix, key string) string {
	if prefix == "" {
		return key
	}
	if strings.HasSuffix(prefix, ".") {
		return prefix + key
	}
	return prefix + "." + key
}

func allowed(path string) bool {
	field := path
	if i := strings.LastIndex(path, "."); i >= 0 {
		field = path[i+1:]
	}
	for _, p := range AllowedFields {
		if field == p || path == p {
			return true
		}
	}
	return false
}

func skipHeader(name string) bool {
	n := strings.ToLower(name)
	switch n {
	case "date", "content-length", "transfer-encoding", "x-litellm-call-id":
		return true
	default:
		return false
	}
}

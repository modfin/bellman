// Package sse holds the line-level parsing shared by every text/event-stream
// consumer in bellman: the provider adapters reading from upstream LLM APIs
// and the client reading from bellmand.
//
// Streams are read line by line. Most lines are "data: <json>" events, but a
// stream may also carry SSE comments (": keepalive"), other SSE fields
// ("event:", "id:", "retry:") and, when an upstream fails mid-stream, a raw
// error body that is not SSE framed at all. Gemini for example drops a
// pretty-printed {"error": {...}} document into the stream on 429/500/503.
// Treating every non-data line as a bare "expected 'data' header" error threw
// away the only evidence of what went wrong; this package classifies the line
// and renders the foreign content into a readable error message instead.
package sse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Kind is what a single stream line is.
type Kind int

const (
	// KindData is a "data:" field; the payload is returned alongside.
	KindData Kind = iota
	// KindIgnore is a comment or an SSE field that carries no payload we act
	// on (event, id, retry).
	KindIgnore
	// KindForeign is a line that is not SSE at all, e.g. one line of a raw
	// JSON error body the upstream wrote instead of an event.
	KindForeign
)

// Classify inspects one line (without its trailing newline) and returns its
// kind. For KindData the payload is the field value with the single optional
// leading space removed, as the SSE specification prescribes.
func Classify(line []byte) (Kind, []byte) {
	if len(line) == 0 {
		return KindIgnore, nil
	}
	if line[0] == ':' {
		return KindIgnore, nil
	}
	colon := bytes.IndexByte(line, ':')
	if colon < 0 {
		return KindForeign, nil
	}
	field := line[:colon]
	value := line[colon+1:]
	if len(value) > 0 && value[0] == ' ' {
		value = value[1:]
	}
	switch string(field) {
	case "data":
		return KindData, value
	case "event", "id", "retry":
		return KindIgnore, nil
	}
	return KindForeign, nil
}

// MaxForeignBody bounds how much of a foreign body Drain collects.
const MaxForeignBody = 64 << 10

// Drain collects the rest of a stream after a foreign line was seen. Upstream
// error documents are often pretty-printed across many lines, so the first
// line alone ("{") says nothing; reading the remainder gives Describe the
// whole document. Reading stops at EOF, at any read error, or after
// MaxForeignBody bytes.
func Drain(first []byte, rest io.Reader) []byte {
	body := append([]byte(nil), first...)
	body = append(body, '\n')
	remaining := MaxForeignBody - len(body)
	if remaining <= 0 {
		return body
	}
	tail, _ := io.ReadAll(io.LimitReader(rest, int64(remaining)))
	return append(body, tail...)
}

type apiError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

type errorEnvelope struct {
	Error *apiError `json:"error"`
}

// Describe renders a foreign body as an error message. It recognises the
// Google and OpenAI style {"error": {"code", "status", "message"}} envelope,
// both bare and wrapped in a JSON array, and otherwise quotes the raw body
// truncated to a readable length.
func Describe(body []byte) string {
	if e := parseAPIError(body); e != nil {
		var b strings.Builder
		b.WriteString("upstream error")
		if e.Code != 0 {
			b.WriteString(" ")
			b.WriteString(strconv.Itoa(e.Code))
		}
		if e.Status != "" {
			b.WriteString(" ")
			b.WriteString(e.Status)
		}
		if e.Message != "" {
			b.WriteString(": ")
			b.WriteString(e.Message)
		}
		return b.String()
	}
	return fmt.Sprintf("unexpected non-sse content in stream: %s", truncate(body, 512))
}

func parseAPIError(body []byte) *apiError {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil
	}
	var single errorEnvelope
	if trimmed[0] == '{' && json.Unmarshal(trimmed, &single) == nil && single.Error != nil {
		return single.Error
	}
	if trimmed[0] == '[' {
		var many []errorEnvelope
		if json.Unmarshal(trimmed, &many) == nil {
			for _, m := range many {
				if m.Error != nil {
					return m.Error
				}
			}
		}
		// A truncated array body ("[{...}," cut by MaxForeignBody or EOF) still
		// deserves a message; try the first object on its own.
		if end := bytes.LastIndexByte(trimmed, '}'); end > 0 {
			if json.Unmarshal(trimmed[1:end+1], &single) == nil && single.Error != nil {
				return single.Error
			}
		}
	}
	return nil
}

func truncate(body []byte, n int) string {
	s := strings.TrimSpace(string(body))
	if len(s) <= n {
		return strconv.Quote(s)
	}
	return strconv.Quote(s[:n]) + fmt.Sprintf("... (%d more bytes)", len(s)-n)
}

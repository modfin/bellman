package vertexai

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
)

// bodyTransport answers every request with the given stream body.
type bodyTransport struct {
	body string
}

func (t bodyTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader(t.body)),
	}, nil
}

func streamBody(t *testing.T, body string) []*gen.StreamResponse {
	t.Helper()
	g := &generator{
		google: &Google{
			config: GoogleConfig{Project: "test-project", Region: "europe-north1"},
			client: &http.Client{Transport: bodyTransport{body: body}},
		},
		request: gen.Request{Model: gen.Model{Name: "gemini-test"}},
	}
	ch, err := g.Stream(prompt.AsUser("hi"))
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	var out []*gen.StreamResponse
	timeout := time.After(5 * time.Second)
	for {
		select {
		case r, ok := <-ch:
			if !ok {
				return out
			}
			out = append(out, r)
		case <-timeout:
			t.Fatalf("stream did not close; got %d events", len(out))
		}
	}
}

func textOf(events []*gen.StreamResponse) string {
	var b strings.Builder
	for _, e := range events {
		if e.Type == gen.TYPE_DELTA {
			b.WriteString(e.Content)
		}
	}
	return b.String()
}

func errorsOf(events []*gen.StreamResponse) []string {
	var errs []string
	for _, e := range events {
		if e.Type == gen.TYPE_ERROR {
			errs = append(errs, e.Content)
		}
	}
	return errs
}

const textChunk = `data: {"candidates":[{"index":0,"content":{"role":"model","parts":[{"text":"Hello"}]}}]}` + "\n\n"

func TestStreamRawErrorDocumentMidStream(t *testing.T) {
	body := textChunk + `{
  "error": {
    "code": 503,
    "message": "The service is currently unavailable.",
    "status": "UNAVAILABLE"
  }
}
`
	events := streamBody(t, body)
	if got := textOf(events); got != "Hello" {
		t.Fatalf("text = %q, want %q", got, "Hello")
	}
	errs := errorsOf(events)
	if len(errs) != 1 {
		t.Fatalf("errors = %v, want exactly one", errs)
	}
	want := "upstream error 503 UNAVAILABLE: The service is currently unavailable."
	if errs[0] != want {
		t.Fatalf("error = %q, want %q", errs[0], want)
	}
	if last := events[len(events)-1]; last.Type != gen.TYPE_EOF {
		t.Fatalf("last event = %v, want EOF", last.Type)
	}
}

func TestStreamErrorInsideDataEvent(t *testing.T) {
	body := textChunk + `data: {"error":{"code":429,"message":"Quota exceeded","status":"RESOURCE_EXHAUSTED"}}` + "\n\n"
	errs := errorsOf(streamBody(t, body))
	if len(errs) != 1 || errs[0] != "upstream error 429 RESOURCE_EXHAUSTED: Quota exceeded" {
		t.Fatalf("errors = %v", errs)
	}
}

func TestStreamIgnoresCommentsAndFields(t *testing.T) {
	body := ": keepalive\n\nevent: message\nid: 1\n" + textChunk + ": another\n\n" +
		`data: {"candidates":[{"index":0,"content":{"role":"model","parts":[{"text":" world"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":2,"totalTokenCount":5}}` + "\n\n"
	events := streamBody(t, body)
	if errs := errorsOf(events); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if got := textOf(events); got != "Hello world" {
		t.Fatalf("text = %q", got)
	}
}

func TestStreamUsageOnlyChunkDoesNotPanic(t *testing.T) {
	body := textChunk +
		`data: {"usageMetadata":{"promptTokenCount":3,"candidatesTokenCount":1,"totalTokenCount":4},"modelVersion":"gemini-test-001"}` + "\n\n"
	events := streamBody(t, body)
	if errs := errorsOf(events); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	var meta int
	for _, e := range events {
		if e.Type == gen.TYPE_METADATA && e.Metadata != nil && e.Metadata.TotalTokens == 4 && e.Metadata.Model == "gemini-test-001" {
			meta++
		}
	}
	if meta != 1 {
		t.Fatalf("metadata events = %d, want 1", meta)
	}
}

func TestStreamPromptBlocked(t *testing.T) {
	body := `data: {"promptFeedback":{"blockReason":"PROHIBITED_CONTENT"},"usageMetadata":{"promptTokenCount":3,"totalTokenCount":3}}` + "\n\n"
	errs := errorsOf(streamBody(t, body))
	if len(errs) != 1 || errs[0] != "prompt blocked: PROHIBITED_CONTENT" {
		t.Fatalf("errors = %v", errs)
	}
}

func TestStreamMalformedChunkIsAnError(t *testing.T) {
	body := textChunk + "data: {not json\n\n"
	errs := errorsOf(streamBody(t, body))
	if len(errs) != 1 || !strings.HasPrefix(errs[0], "could not unmarshal stream chunk") {
		t.Fatalf("errors = %v", errs)
	}
}

func TestStreamHTMLFromProxy(t *testing.T) {
	body := textChunk + "<html><body><h1>502 Bad Gateway</h1></body></html>\n"
	errs := errorsOf(streamBody(t, body))
	if len(errs) != 1 || !strings.Contains(errs[0], "502 Bad Gateway") {
		t.Fatalf("errors = %v", errs)
	}
}

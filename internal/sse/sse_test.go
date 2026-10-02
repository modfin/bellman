package sse

import (
	"strings"
	"testing"
)

func TestClassify(t *testing.T) {
	cases := []struct {
		line    string
		kind    Kind
		payload string
	}{
		{`data: {"a":1}`, KindData, `{"a":1}`},
		{`data:{"a":1}`, KindData, `{"a":1}`},
		{`data:  two spaces`, KindData, ` two spaces`},
		{`data: [DONE]`, KindData, `[DONE]`},
		{`: keepalive`, KindIgnore, ""},
		{`:`, KindIgnore, ""},
		{`event: message`, KindIgnore, ""},
		{`id: 42`, KindIgnore, ""},
		{`retry: 1000`, KindIgnore, ""},
		{``, KindIgnore, ""},
		{`{`, KindForeign, ""},
		{`{"error": {"code": 503}}`, KindForeign, ""},
		{`  "code": 503,`, KindForeign, ""},
		{`<html>`, KindForeign, ""},
		{`[`, KindForeign, ""},
	}
	for _, c := range cases {
		kind, payload := Classify([]byte(c.line))
		if kind != c.kind {
			t.Errorf("Classify(%q) kind = %v, want %v", c.line, kind, c.kind)
		}
		if string(payload) != c.payload {
			t.Errorf("Classify(%q) payload = %q, want %q", c.line, payload, c.payload)
		}
	}
}

func TestDrain(t *testing.T) {
	rest := strings.NewReader("  \"error\": {\n    \"code\": 503\n  }\n}\n")
	body := Drain([]byte("{"), rest)
	want := "{\n  \"error\": {\n    \"code\": 503\n  }\n}\n"
	if string(body) != want {
		t.Fatalf("Drain = %q, want %q", body, want)
	}
}

func TestDrainBounded(t *testing.T) {
	rest := strings.NewReader(strings.Repeat("x", 2*MaxForeignBody))
	body := Drain([]byte("{"), rest)
	if len(body) != MaxForeignBody {
		t.Fatalf("Drain collected %d bytes, want %d", len(body), MaxForeignBody)
	}
}

func TestDescribe(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			name: "google pretty printed",
			body: "{\n  \"error\": {\n    \"code\": 503,\n    \"message\": \"The service is currently unavailable.\",\n    \"status\": \"UNAVAILABLE\"\n  }\n}\n",
			want: "upstream error 503 UNAVAILABLE: The service is currently unavailable.",
		},
		{
			name: "compact",
			body: `{"error":{"code":429,"message":"Resource exhausted","status":"RESOURCE_EXHAUSTED"}}`,
			want: "upstream error 429 RESOURCE_EXHAUSTED: Resource exhausted",
		},
		{
			name: "array wrapped",
			body: `[{"error":{"code":500,"message":"Internal error encountered.","status":"INTERNAL"}}]`,
			want: "upstream error 500 INTERNAL: Internal error encountered.",
		},
		{
			name: "truncated array",
			body: `[{"error":{"code":500,"message":"Internal error encountered.","status":"INTERNAL"}},`,
			want: "upstream error 500 INTERNAL: Internal error encountered.",
		},
		{
			name: "message only",
			body: `{"error":{"message":"boom"}}`,
			want: "upstream error: boom",
		},
		{
			name: "html",
			body: "<html><body>502 Bad Gateway</body></html>\n",
			want: `unexpected non-sse content in stream: "<html><body>502 Bad Gateway</body></html>"`,
		},
		{
			name: "json without error envelope",
			body: `{"candidates": []}`,
			want: `unexpected non-sse content in stream: "{\"candidates\": []}"`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Describe([]byte(c.body))
			if got != c.want {
				t.Fatalf("Describe = %q, want %q", got, c.want)
			}
		})
	}
}

func TestDescribeTruncatesLongBodies(t *testing.T) {
	got := Describe([]byte(strings.Repeat("y", 1000)))
	if !strings.Contains(got, "... (488 more bytes)") {
		t.Fatalf("Describe did not truncate: %q", got)
	}
}

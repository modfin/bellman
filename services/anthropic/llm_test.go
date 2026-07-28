package anthropic

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/modfin/bellman/models"
	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/tools"
)

func testGenerator(t *testing.T, req gen.Request) *generator {
	t.Helper()
	req.Model = GenModel_4_5_haiku_latest
	return &generator{anthropic: New("test-key"), request: req}
}

func conversation() []prompt.Prompt {
	return []prompt.Prompt{
		prompt.AsUser("what is the price of VOLV-B.ST?"),
		prompt.AsToolCall("call-1", "get_price", []byte(`{"symbol":"VOLV-B.ST"}`)),
		prompt.AsToolResponse("call-1", "get_price", `{"price":250}`),
	}
}

func withTools() []tools.Tool {
	return []tools.Tool{
		tools.NewTool("get_price", tools.WithArgSchema(tools.EmptyArgs{})),
		tools.NewTool("get_volume", tools.WithArgSchema(tools.EmptyArgs{})),
	}
}

func TestPromptCacheDisabledSetsNoBreakpoints(t *testing.T) {
	g := testGenerator(t, gen.Request{SystemPrompt: "you look up prices", Tools: withTools()})

	_, model, err := g.prompt(conversation()...)
	if err != nil {
		t.Fatalf("prompt() error = %v", err)
	}

	for i, tool := range model.Tools {
		if tool.CacheControl != nil {
			t.Errorf("Tools[%d] has a cache breakpoint without PromptCache", i)
		}
	}
	for i, block := range model.System {
		if block.CacheControl != nil {
			t.Errorf("System[%d] has a cache breakpoint without PromptCache", i)
		}
	}
	for i, msg := range model.Messages {
		for j, c := range msg.Content {
			if c.CacheControl != nil {
				t.Errorf("Messages[%d].Content[%d] has a cache breakpoint without PromptCache", i, j)
			}
		}
	}
}

func TestPromptCacheSetsBreakpointsOnReusablePrefix(t *testing.T) {
	g := testGenerator(t, gen.Request{SystemPrompt: "you look up prices", Tools: withTools(), PromptCache: true})

	_, model, err := g.prompt(conversation()...)
	if err != nil {
		t.Fatalf("prompt() error = %v", err)
	}

	if n := len(model.Tools); n == 0 || model.Tools[n-1].CacheControl == nil {
		t.Error("want a cache breakpoint on the last tool definition")
	}
	for i := 0; i < len(model.Tools)-1; i++ {
		if model.Tools[i].CacheControl != nil {
			t.Errorf("Tools[%d] should not carry a breakpoint, only the last one", i)
		}
	}
	if n := len(model.System); n == 0 || model.System[n-1].CacheControl == nil {
		t.Error("want a cache breakpoint on the system prompt")
	}

	// The breakpoint on the conversation has to sit on the very last block, so
	// the next turn reads this turn's prefix out of the cache.
	last := model.Messages[len(model.Messages)-1]
	if n := len(last.Content); n == 0 || last.Content[n-1].CacheControl == nil {
		t.Errorf("want a cache breakpoint on the last block of the conversation, got %+v", last)
	}
	breakpoints := 0
	for _, msg := range model.Messages {
		for _, c := range msg.Content {
			if c.CacheControl != nil {
				breakpoints++
			}
		}
	}
	if breakpoints != 1 {
		t.Errorf("got %d breakpoints in the conversation, want exactly 1 (max 4 in total)", breakpoints)
	}
}

// cache_control is rejected on thinking blocks, so a conversation whose final
// block is thinking must anchor the breakpoint further back.
func TestPromptCacheSkipsThinkingBlocks(t *testing.T) {
	g := testGenerator(t, gen.Request{PromptCache: true})

	_, model, err := g.prompt(
		prompt.AsUser("hi"),
		prompt.AsAssistant("hello"),
		prompt.AsThinking("still pondering", []byte("sig-1"), ""),
	)
	if err != nil {
		t.Fatalf("prompt() error = %v", err)
	}

	last := model.Messages[len(model.Messages)-1]
	for _, c := range last.Content {
		if c.Type == "thinking" && c.CacheControl != nil {
			t.Fatal("cache breakpoint placed on a thinking block")
		}
	}
	var marked int
	for _, c := range last.Content {
		if c.CacheControl != nil {
			marked++
			if c.Type != "text" {
				t.Errorf("breakpoint placed on a %q block, want the text block", c.Type)
			}
		}
	}
	if marked != 1 {
		t.Errorf("got %d marked blocks, want 1", marked)
	}
}

// The system prompt is sent as text blocks (rather than a bare string) so it can
// carry a cache breakpoint.
func TestSystemPromptMarshalsAsTextBlocks(t *testing.T) {
	g := testGenerator(t, gen.Request{SystemPrompt: "you look up prices"})

	_, model, err := g.prompt(prompt.AsUser("hi"))
	if err != nil {
		t.Fatalf("prompt() error = %v", err)
	}

	body, err := json.Marshal(model)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	var sent struct {
		System []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"system"`
	}
	if err := json.Unmarshal(body, &sent); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	if len(sent.System) != 1 || sent.System[0].Type != "text" || sent.System[0].Text != "you look up prices" {
		t.Fatalf("system = %+v, want one text block with the system prompt", sent.System)
	}

	// An empty system prompt must stay off the wire entirely.
	g = testGenerator(t, gen.Request{})
	_, model, err = g.prompt(prompt.AsUser("hi"))
	if err != nil {
		t.Fatalf("prompt() error = %v", err)
	}
	body, err = json.Marshal(model)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		t.Fatalf("decode request: %v", err)
	}
	if _, ok := raw["system"]; ok {
		t.Error("empty system prompt was still sent")
	}
}

// input_tokens excludes the cache counters, so the reported input has to add
// them back for CachedTokens to be a subset of InputTokens.
func TestUsageAccountsForCacheTokens(t *testing.T) {
	usage := anthropicUsage{
		InputTokens:              10,
		OutputTokens:             7,
		CacheReadInputTokens:     800,
		CacheCreationInputTokens: 200,
	}

	m := usageToMetadata("claude", usage)
	if m.InputTokens != 1010 {
		t.Errorf("InputTokens = %d, want 1010", m.InputTokens)
	}
	if m.CachedTokens != 800 {
		t.Errorf("CachedTokens = %d, want 800", m.CachedTokens)
	}
	if m.CacheWriteTokens != 200 {
		t.Errorf("CacheWriteTokens = %d, want 200", m.CacheWriteTokens)
	}
	if m.TotalTokens != 1017 {
		t.Errorf("TotalTokens = %d, want 1017", m.TotalTokens)
	}
}

// streamTransport serves a canned SSE body for any request. Stream() posts via
// http.DefaultClient, so the test swaps its transport for the duration; keep
// these subtests serial (no t.Parallel).
type streamTransport struct {
	body string
}

func (s *streamTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(strings.NewReader(s.body)),
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
	}, nil
}

func withStreamBody(t *testing.T, body string) {
	t.Helper()
	original := http.DefaultClient.Transport
	http.DefaultClient.Transport = &streamTransport{body: body}
	t.Cleanup(func() { http.DefaultClient.Transport = original })
}

func sse(events ...string) string {
	var b strings.Builder
	for _, e := range events {
		b.WriteString("data: " + e + "\n\n")
	}
	return b.String()
}

// Anthropic reports usage in pieces: the input and cache counters arrive on
// message_start (where output_tokens is a placeholder) and the final output
// count on message_delta. Consumers keep the last metadata frame, so every
// frame has to carry the running total rather than just the piece that changed.
func TestStreamMetadataAccumulatesUsage(t *testing.T) {
	withStreamBody(t, sse(
		`{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-haiku-4-5-20251001","content":[],"usage":{"input_tokens":25,"cache_read_input_tokens":800,"cache_creation_input_tokens":200,"output_tokens":1}}}`,
		`{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`,
		`{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hi"}}`,
		`{"type":"content_block_stop","index":0}`,
		`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":15}}`,
		`{"type":"message_stop"}`,
	))

	g := testGenerator(t, gen.Request{PromptCache: true})
	stream, err := g.Stream(prompt.AsUser("hi"))
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	var last *models.Metadata
	var frames int
	for r := range stream {
		if r.Type == gen.TYPE_ERROR {
			t.Fatalf("stream error = %v", r.Content)
		}
		if r.Type == gen.TYPE_METADATA && r.Metadata != nil {
			frames++
			last = r.Metadata
		}
	}

	if frames == 0 || last == nil {
		t.Fatal("no metadata frames in stream")
	}
	if last.InputTokens != 1025 {
		t.Errorf("InputTokens = %d, want 1025 (25 uncached + 800 read + 200 written)", last.InputTokens)
	}
	if last.OutputTokens != 15 {
		t.Errorf("OutputTokens = %d, want 15 (message_start's 1 is a placeholder)", last.OutputTokens)
	}
	if last.CachedTokens != 800 {
		t.Errorf("CachedTokens = %d, want 800", last.CachedTokens)
	}
	if last.CacheWriteTokens != 200 {
		t.Errorf("CacheWriteTokens = %d, want 200", last.CacheWriteTokens)
	}
	if last.TotalTokens != 1040 {
		t.Errorf("TotalTokens = %d, want 1040", last.TotalTokens)
	}
	if last.Model != "claude-haiku-4-5-20251001" {
		t.Errorf("Model = %q, want the model the response resolved to", last.Model)
	}
}

// Every frame, not just the last, has to be a complete running total: consumers
// that stop reading early (or a proxy that forwards frames as they arrive) must
// never see the input drop back to zero.
func TestStreamMetadataFramesAreNeverPartial(t *testing.T) {
	withStreamBody(t, sse(
		`{"type":"message_start","message":{"id":"msg_1","type":"message","role":"assistant","model":"claude-haiku-4-5-20251001","content":[],"usage":{"input_tokens":25,"output_tokens":1}}}`,
		`{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":15}}`,
		`{"type":"message_stop"}`,
	))

	g := testGenerator(t, gen.Request{})
	stream, err := g.Stream(prompt.AsUser("hi"))
	if err != nil {
		t.Fatalf("Stream() error = %v", err)
	}

	for r := range stream {
		if r.Type != gen.TYPE_METADATA || r.Metadata == nil {
			continue
		}
		if r.Metadata.InputTokens != 25 {
			t.Errorf("metadata frame reports InputTokens = %d, want 25 in every frame", r.Metadata.InputTokens)
		}
		if r.Metadata.TotalTokens < r.Metadata.InputTokens {
			t.Errorf("metadata frame TotalTokens = %d is below InputTokens = %d", r.Metadata.TotalTokens, r.Metadata.InputTokens)
		}
	}
}

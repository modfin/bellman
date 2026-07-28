package vertexai

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/tools"
)

type stubTransport struct {
	body    string
	request []byte
}

func (s *stubTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.Body != nil {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		s.request = b
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader([]byte(s.body))),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
	}, nil
}

func stubClient(t *testing.T, response string) (*Google, *stubTransport) {
	t.Helper()
	st := &stubTransport{body: response}
	return &Google{
		config: GoogleConfig{Project: "test-project", Region: "europe-north1"},
		client: &http.Client{Transport: st},
	}, st
}

const usage = `"usageMetadata":{"promptTokenCount":100,"candidatesTokenCount":10,"thoughtsTokenCount":5,"totalTokenCount":115,"cachedContentTokenCount":80}`

// Gemini can return the turn's thoughtSignature in a trailing part with no text
// and no functionCall. Prompt() must attach it to the first tool call of the
// turn, the same way Stream() does — dropping it loses the bytes the next
// request has to echo back.
func TestPromptAttachesClosureSignatureToToolCall(t *testing.T) {
	client, _ := stubClient(t, `{"candidates":[{"content":{"role":"model","parts":[
		{"text":"i should look it up","thought":true},
		{"functionCall":{"name":"get_price","args":{"symbol":"VOLV-B.ST"}}},
		{"text":"","thoughtSignature":"CLOSURE-SIG"}
	]},"finishReason":"STOP"}],`+usage+`}`)

	priceTool := tools.NewTool("get_price", tools.WithArgSchema(tools.EmptyArgs{}))
	g := client.Generator(gen.WithModel(GenModel_gemini_2_5_flash_latest), gen.WithTools(priceTool))

	res, err := g.Prompt(prompt.AsUser("what is the price of VOLV-B.ST?"))
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}

	if len(res.Tools) != 1 {
		t.Fatalf("len(Tools) = %d, want 1", len(res.Tools))
	}
	if res.Tools[0].ID == "" {
		t.Error("tool call has no ID, so its tool response cannot be paired with it")
	}
	if res.Tools[0].Ref == nil {
		t.Error("tool call has no Ref")
	}

	if len(res.Turn) != 1 {
		t.Fatalf("len(Turn) = %d, want 1 (unsigned thinking dropped, tool call kept): %+v", len(res.Turn), res.Turn)
	}
	call := res.Turn[0]
	if call.Role != prompt.ToolCallRole {
		t.Fatalf("Turn[0].Role = %q, want %q", call.Role, prompt.ToolCallRole)
	}
	if got := string(call.Replay); got != "CLOSURE-SIG" {
		t.Errorf("Turn[0].Replay = %q, want the closure signature", got)
	}
	if call.ToolCall.ToolCallID != res.Tools[0].ID {
		t.Errorf("Turn[0] id %q != Tools[0] id %q", call.ToolCall.ToolCallID, res.Tools[0].ID)
	}
	if len(res.Thinking) != 1 || res.Thinking[0] != "i should look it up" {
		t.Errorf("Thinking = %+v, want the thought text to still be reported", res.Thinking)
	}
}

// A thinking block without a signature cannot be replayed, so it must not enter
// Turn — Stream() already refuses to emit those.
func TestPromptSkipsUnsignedThinkingInTurn(t *testing.T) {
	client, _ := stubClient(t, `{"candidates":[{"content":{"role":"model","parts":[
		{"text":"unsigned thought","thought":true},
		{"text":"signed thought","thought":true,"thoughtSignature":"SIG-T"},
		{"text":"the answer","thoughtSignature":"SIG-A"}
	]},"finishReason":"STOP"}],`+usage+`}`)

	g := client.Generator(gen.WithModel(GenModel_gemini_2_5_flash_latest))
	res, err := g.Prompt(prompt.AsUser("hi"))
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}

	if len(res.Turn) != 2 {
		t.Fatalf("len(Turn) = %d, want 2 (signed thinking + text): %+v", len(res.Turn), res.Turn)
	}
	if res.Turn[0].Role != prompt.ThinkingRole || string(res.Turn[0].Replay) != "SIG-T" {
		t.Errorf("Turn[0] = %+v, want the signed thinking block", res.Turn[0])
	}
	if res.Turn[1].Role != prompt.AssistantRole || string(res.Turn[1].Replay) != "SIG-A" {
		t.Errorf("Turn[1] = %+v, want the signed assistant text", res.Turn[1])
	}
	if len(res.Thinking) != 2 {
		t.Errorf("len(Thinking) = %d, want 2 — both thoughts are still visible text", len(res.Thinking))
	}
}

func TestPromptReportsCachedTokens(t *testing.T) {
	client, _ := stubClient(t, `{"candidates":[{"content":{"role":"model","parts":[{"text":"hi"}]},"finishReason":"STOP"}],`+usage+`}`)

	g := client.Generator(gen.WithModel(GenModel_gemini_2_5_flash_latest))
	res, err := g.Prompt(prompt.AsUser("hi"))
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}
	if res.Metadata.CachedTokens != 80 {
		t.Errorf("Metadata.CachedTokens = %d, want 80", res.Metadata.CachedTokens)
	}
	if res.Metadata.InputTokens != 100 {
		t.Errorf("Metadata.InputTokens = %d, want 100 (cached tokens are a subset)", res.Metadata.InputTokens)
	}
}

// Unsigned thought parts cannot be validated by the API, so the request builder
// must not send them back regardless of where the prompt came from.
func TestRequestDropsUnsignedThinkingPrompts(t *testing.T) {
	client, st := stubClient(t, `{"candidates":[{"content":{"role":"model","parts":[{"text":"hi"}]},"finishReason":"STOP"}],`+usage+`}`)

	g := client.Generator(gen.WithModel(GenModel_gemini_2_5_flash_latest))
	_, err := g.Prompt(
		prompt.AsUser("hi"),
		prompt.AsThinking("unsigned", nil, ""),
		prompt.AsThinking("signed", []byte("SIG-T"), ""),
		prompt.AsAssistant("previous answer"),
		prompt.AsUser("and now?"),
	)
	if err != nil {
		t.Fatalf("Prompt() error = %v", err)
	}

	var sent genRequest
	if err := json.Unmarshal(st.request, &sent); err != nil {
		t.Fatalf("could not decode sent request: %v", err)
	}

	var thoughts []genRequestContentPart
	for _, c := range sent.Contents {
		for _, p := range c.Parts {
			if p.Thought {
				thoughts = append(thoughts, p)
			}
		}
	}
	if len(thoughts) != 1 {
		t.Fatalf("sent %d thought parts, want 1 (the signed one): %+v", len(thoughts), thoughts)
	}
	if thoughts[0].ThoughtSignature != "SIG-T" {
		t.Errorf("thought part signature = %q, want SIG-T", thoughts[0].ThoughtSignature)
	}
}

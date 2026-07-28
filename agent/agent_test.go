package agent_test

import (
	"context"
	"errors"
	"testing"

	"github.com/modfin/bellman/agent"
	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/tools"
)

// stubPrompter plays back a canned sequence of responses and records the
// conversation it was handed on every call, so the agent loop's history
// bookkeeping can be asserted without a provider.
type stubPrompter struct {
	responses []*gen.Response
	calls     int
	seen      [][]prompt.Prompt
}

func (s *stubPrompter) SetRequest(gen.Request) {}

func (s *stubPrompter) Prompt(prompts ...prompt.Prompt) (*gen.Response, error) {
	s.seen = append(s.seen, append([]prompt.Prompt(nil), prompts...))
	if s.calls >= len(s.responses) {
		return nil, errors.New("stub: no responses left")
	}
	r := s.responses[s.calls]
	s.calls++
	return r, nil
}

func (s *stubPrompter) Stream(...prompt.Prompt) (<-chan *gen.StreamResponse, error) {
	return nil, errors.New("stub: streaming not supported")
}

type priceResult struct {
	Price float64 `json:"price"`
}

func priceTool(t *testing.T) tools.Tool {
	t.Helper()
	return tools.NewTool("get_price",
		tools.WithDescription("get a price"),
		tools.WithArgSchema(tools.EmptyArgs{}),
		tools.WithFunction(func(ctx context.Context, call tools.Call) (string, error) {
			return `{"price":42.5}`, nil
		}),
	)
}

func stubGenerator(p gen.Prompter, tool tools.Tool) *gen.Generator {
	g := &gen.Generator{
		Prompter: p,
		Request:  gen.Request{Model: gen.Model{Provider: "stub", Name: "stub"}},
	}
	return g.SetTools(tool)
}

// The terminating assistant turn must end up in Result.Prompts, otherwise the
// returned conversation cannot be used as the history for a follow-up turn —
// the model's own answer, and the signature attached to it, would be missing.
func TestRunResultPromptsIncludeFinalAssistantTurn(t *testing.T) {
	tool := priceTool(t)
	stub := &stubPrompter{responses: []*gen.Response{
		{
			Tools: []tools.Call{{ID: "call-1", Name: "get_price", Argument: []byte(`{}`), Ref: &tool}},
			Turn: []prompt.Prompt{
				prompt.AsThinking("i should look up the price", []byte("sig-1"), "r1"),
				prompt.AsToolCall("call-1", "get_price", []byte(`{}`)),
			},
		},
		{
			Texts: []string{`{"price":42.5}`},
			Turn: []prompt.Prompt{
				prompt.AsThinking("the tool gave me the price", []byte("sig-2"), "r2"),
				prompt.AsAssistant(`{"price":42.5}`),
			},
		},
	}}

	res, err := agent.Run[priceResult](5, 1, stubGenerator(stub, tool), prompt.AsUser("what is the price?"))
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if res.Result.Price != 42.5 {
		t.Fatalf("Result.Price = %v, want 42.5", res.Result.Price)
	}

	wantRoles := []prompt.Role{
		prompt.UserRole,
		prompt.ThinkingRole,
		prompt.ToolCallRole,
		prompt.ToolResponseRole,
		prompt.ThinkingRole,
		prompt.AssistantRole,
	}
	if len(res.Prompts) != len(wantRoles) {
		t.Fatalf("len(Result.Prompts) = %d, want %d: %+v", len(res.Prompts), len(wantRoles), res.Prompts)
	}
	for i, want := range wantRoles {
		if res.Prompts[i].Role != want {
			t.Errorf("Result.Prompts[%d].Role = %q, want %q", i, res.Prompts[i].Role, want)
		}
	}

	last := res.Prompts[len(res.Prompts)-1]
	if last.Text != `{"price":42.5}` {
		t.Errorf("final prompt text = %q, want the assistant answer", last.Text)
	}
	if got := string(res.Prompts[4].Replay); got != "sig-2" {
		t.Errorf("final thinking Replay = %q, want %q", got, "sig-2")
	}
}

// Run must not write into the caller's slice. Passing prompts... aliases the
// caller's backing array, so appending the assistant turn can clobber elements
// past len that the caller (or another slice of the same array) still owns.
func TestRunDoesNotMutateCallerPrompts(t *testing.T) {
	tool := priceTool(t)
	stub := &stubPrompter{responses: []*gen.Response{
		{
			Tools: []tools.Call{{ID: "call-1", Name: "get_price", Argument: []byte(`{}`), Ref: &tool}},
			Turn:  []prompt.Prompt{prompt.AsToolCall("call-1", "get_price", []byte(`{}`))},
		},
		{
			Texts: []string{`{"price":42.5}`},
			Turn:  []prompt.Prompt{prompt.AsAssistant(`{"price":42.5}`)},
		},
	}}

	sentinel := prompt.AsUser("SENTINEL")
	backing := make([]prompt.Prompt, 4)
	backing[0] = prompt.AsUser("what is the price?")
	for i := 1; i < len(backing); i++ {
		backing[i] = sentinel
	}
	caller := backing[:1]

	_, err := agent.Run[priceResult](5, 1, stubGenerator(stub, tool), caller...)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	for i := 1; i < len(backing); i++ {
		if backing[i].Text != sentinel.Text {
			t.Fatalf("Run wrote into the caller's backing array at index %d: %+v", i, backing[i])
		}
	}
	if len(caller) != 1 {
		t.Fatalf("caller slice length changed to %d", len(caller))
	}
}

// RunWithToolsOnly extracts its result from a synthetic tool call that never
// gets a tool response, so that call must not be replayed as history — but the
// assistant text of the final turn is safe to keep.
func TestRunWithToolsOnlyPromptsAreReplaySafe(t *testing.T) {
	tool := priceTool(t)
	resultCall := tools.Call{ID: "call-2", Name: "__return_result_tool__", Argument: []byte(`{"price":42.5}`)}
	stub := &stubPrompter{responses: []*gen.Response{
		{
			Tools: []tools.Call{{ID: "call-1", Name: "get_price", Argument: []byte(`{}`), Ref: &tool}},
			Turn:  []prompt.Prompt{prompt.AsToolCall("call-1", "get_price", []byte(`{}`))},
		},
		{
			Tools: []tools.Call{resultCall},
			Turn: []prompt.Prompt{
				prompt.AsThinking("wrapping up", []byte("sig-2"), "r2"),
				prompt.AsAssistant("here is the price"),
				prompt.AsToolCall(resultCall.ID, resultCall.Name, resultCall.Argument),
			},
		},
	}}

	res, err := agent.RunWithToolsOnly[priceResult](5, 1, stubGenerator(stub, tool), prompt.AsUser("what is the price?"))
	if err != nil {
		t.Fatalf("RunWithToolsOnly() error = %v", err)
	}
	if res.Result.Price != 42.5 {
		t.Fatalf("Result.Price = %v, want 42.5", res.Result.Price)
	}

	for i, p := range res.Prompts {
		if p.Role == prompt.ToolCallRole && p.ToolCall != nil && p.ToolCall.Name == "__return_result_tool__" {
			t.Fatalf("Result.Prompts[%d] replays the synthetic result tool call, which has no tool response", i)
		}
	}
	last := res.Prompts[len(res.Prompts)-1]
	if last.Role != prompt.AssistantRole || last.Text != "here is the price" {
		t.Fatalf("final prompt = %+v, want the assistant text of the last turn", last)
	}
}

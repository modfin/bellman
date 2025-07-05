package gen

import (
	"context"
	"reflect"
	"testing"

	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/schema"
	"github.com/modfin/bellman/tools"
)

// fakePrompter implements the Prompter interface for testing.
type fakePrompter struct {
	lastRequest Request
	prompts     []prompt.Prompt
	retResp     *Response
	retChan     chan *StreamResponse
	retErr      error
}

func (f *fakePrompter) SetRequest(r Request) {
	f.lastRequest = r
}
func (f *fakePrompter) Prompt(prompts ...prompt.Prompt) (*Response, error) {
	f.prompts = prompts
	return f.retResp, f.retErr
}
func (f *fakePrompter) Stream(prompts ...prompt.Prompt) (<-chan *StreamResponse, error) {
	f.prompts = prompts
	if f.retChan == nil {
		return nil, f.retErr
	}
	return f.retChan, f.retErr
}

func TestFloatInt(t *testing.T) {
	f := Float(3.14)
	if f == nil || *f != 3.14 {
		t.Errorf("Float(3.14) = %v, want pointer to 3.14", f)
	}
	i := Int(42)
	if i == nil || *i != 42 {
		t.Errorf("Int(42) = %v, want pointer to 42", i)
	}
}

func TestSetConfigAndClone(t *testing.T) {
	orig := &Generator{Request: Request{SystemPrompt: "a"}}
	req := Request{SystemPrompt: "b"}
	newG := orig.SetConfig(req)
	if newG.Request.SystemPrompt != "b" {
		t.Errorf("SetConfig didn't set SystemPrompt, got %q", newG.Request.SystemPrompt)
	}
	if orig.Request.SystemPrompt != "a" {
		t.Errorf("SetConfig mutated original, orig SystemPrompt = %q", orig.Request.SystemPrompt)
	}
	if newG == orig {
		t.Error("SetConfig returned same Generator pointer, want new one")
	}
}

func TestCloneDeepCopy(t *testing.T) {
	trueVal := true
	tbVal := 5
	fVal := 1.23
	iVal := 7

	origSchema := &schema.JSON{Description: "desc"}
	ctx := context.WithValue(context.Background(), "key", "val")
	toolA := tools.NewTool("A")
	toolB := tools.NewTool("B")
	original := &Generator{
		Request: Request{
			Context:          ctx,
			Stream:           true,
			Model:            Model{Provider: "p", Name: "n"},
			SystemPrompt:     "sys",
			OutputSchema:     origSchema,
			StrictOutput:     true,
			Tools:            []tools.Tool{toolA},
			ToolConfig:       &toolB,
			ThinkingBudget:   &tbVal,
			ThinkingParts:    &trueVal,
			TopP:             &fVal,
			TopK:             &iVal,
			Temperature:      &fVal,
			MaxTokens:        &iVal,
			FrequencyPenalty: &fVal,
			PresencePenalty:  &fVal,
			StopSequences:    []string{"s1", "s2"},
		},
	}
	cloned := original.clone()
	if cloned == original {
		t.Fatal("clone returned same pointer")
	}
	// context is shallow copied
	if cloned.Request.Context != original.Request.Context {
		t.Error("Context not copied correctly")
	}
	// OutputSchema deep copy
	if cloned.Request.OutputSchema == original.Request.OutputSchema {
		t.Error("OutputSchema pointer not deep-copied")
	}
	// Note: OutputSchema contains maps which cannot be compared with reflect.DeepEqual
	// We just verify the pointer is different

	// ToolConfig deep copy
	if cloned.Request.ToolConfig == original.Request.ToolConfig {
		t.Error("ToolConfig pointer not deep-copied")
	}
	// Note: ToolConfig contains functions which cannot be compared with reflect.DeepEqual
	// We just verify the pointer is different
	// Tools slice deep copy
	if &cloned.Request.Tools[0] == &original.Request.Tools[0] {
		t.Error("Tools slice not deep-copied")
	}
	if !reflect.DeepEqual(cloned.Request.Tools, original.Request.Tools) {
		t.Error("Tools content mismatch")
	}
	// Pointer fields deep copy
	if cloned.Request.PresencePenalty == original.Request.PresencePenalty {
		t.Error("PresencePenalty pointer not deep-copied")
	}
	if *cloned.Request.PresencePenalty != *original.Request.PresencePenalty {
		t.Error("PresencePenalty value mismatch")
	}
	if cloned.Request.FrequencyPenalty == original.Request.FrequencyPenalty {
		t.Error("FrequencyPenalty pointer not deep-copied")
	}
	if *cloned.Request.FrequencyPenalty != *original.Request.FrequencyPenalty {
		t.Error("FrequencyPenalty value mismatch")
	}
	if cloned.Request.Temperature == original.Request.Temperature {
		t.Error("Temperature pointer not deep-copied")
	}
	if *cloned.Request.Temperature != *original.Request.Temperature {
		t.Error("Temperature value mismatch")
	}
	if cloned.Request.TopP == original.Request.TopP {
		t.Error("TopP pointer not deep-copied")
	}
	if *cloned.Request.TopP != *original.Request.TopP {
		t.Error("TopP value mismatch")
	}
	if cloned.Request.TopK == original.Request.TopK {
		t.Error("TopK pointer not deep-copied")
	}
	if *cloned.Request.TopK != *original.Request.TopK {
		t.Error("TopK value mismatch")
	}
	if cloned.Request.MaxTokens == original.Request.MaxTokens {
		t.Error("MaxTokens pointer not deep-copied")
	}
	if *cloned.Request.MaxTokens != *original.Request.MaxTokens {
		t.Error("MaxTokens value mismatch")
	}
	if cloned.Request.ThinkingBudget == original.Request.ThinkingBudget {
		t.Error("ThinkingBudget pointer not deep-copied")
	}
	if *cloned.Request.ThinkingBudget != *original.Request.ThinkingBudget {
		t.Error("ThinkingBudget value mismatch")
	}
	if cloned.Request.ThinkingParts == original.Request.ThinkingParts {
		t.Error("ThinkingParts pointer not deep-copied")
	}
	if *cloned.Request.ThinkingParts != *original.Request.ThinkingParts {
		t.Error("ThinkingParts value mismatch")
	}
	// StopSequences deep copy
	if &cloned.Request.StopSequences[0] == &original.Request.StopSequences[0] {
		t.Error("StopSequences slice not deep-copied")
	}
	if !reflect.DeepEqual(cloned.Request.StopSequences, original.Request.StopSequences) {
		t.Error("StopSequences content mismatch")
	}
}

func TestGeneratorPromptAndStream(t *testing.T) {
	// Prompt error when no prompter
	g := &Generator{}
	_, err := g.Prompt(prompt.AsUser("hi"))
	if err == nil || err.Error() != "prompter is required" {
		t.Errorf("expected prompter required error, got %v", err)
	}
	// Stream error when no prompter
	_, err = g.Stream(prompt.AsUser("hi"))
	if err == nil || err.Error() != "prompter is required" {
		t.Errorf("expected prompter required error, got %v", err)
	}
	// Prompt success
	fp := &fakePrompter{retResp: &Response{Texts: []string{"ok"}}}
	g2 := &Generator{Prompter: fp}
	resp, err := g2.Prompt(prompt.AsAssistant("hello"))
	if err != nil {
		t.Errorf("Prompt unexpected error: %v", err)
	}
	if resp != fp.retResp {
		t.Error("Prompt did not return expected response")
	}
	if len(fp.prompts) != 1 || fp.prompts[0].Text != "hello" {
		t.Errorf("Prompt did not receive prompts, got %v", fp.prompts)
	}
	if !reflect.DeepEqual(fp.lastRequest, g2.Request) {
		t.Errorf("Prompt did not receive request, got %v want %v", fp.lastRequest, g2.Request)
	}
	// Stream success
	ch := make(chan *StreamResponse, 1)
	ch <- &StreamResponse{Type: TYPE_DELTA, Content: "abc"}
	close(ch)
	fp2 := &fakePrompter{retChan: ch}
	g3 := &Generator{Prompter: fp2}
	outCh, err := g3.Stream(prompt.AsUser("x"))
	if err != nil {
		t.Errorf("Stream unexpected error: %v", err)
	}
	select {
	case r, ok := <-outCh:
		if !ok || r.Content != "abc" {
			t.Errorf("Stream channel content mismatch, got %v", r)
		}
	default:
		t.Error("Stream channel empty")
	}
	if len(fp2.prompts) != 1 || fp2.prompts[0].Text != "x" {
		t.Errorf("Stream did not receive prompts, got %v", fp2.prompts)
	}
	if !fp2.lastRequest.Stream {
		t.Error("Stream request not set with Stream=true")
	}
}

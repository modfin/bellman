package testsuite

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/schema"
	"github.com/modfin/bellman/tools"
)

func testHello(g *gen.Generator) func(*testing.T) {
	return func(t *testing.T) {
		res, err := g.Prompt(prompt.AsUser("Say 'Hello, World!'"))
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}

		txt, err := res.AsText()
		if err != nil {
			t.Fatalf("AsText() error = %v", err)
		}

		low := strings.ToLower(txt)
		if !strings.Contains(low, "hello") || !strings.Contains(low, "world") {
			t.Fatalf("expected response %q to contain 'hello' and 'world'", txt)
		}
	}
}

func testTool(g *gen.Generator) func(*testing.T) {
	return func(t *testing.T) {
		type Args struct {
			Name string `json:"name" json-description:"the name of a person" json-enum:"Othello,Macbeth,Juliet"`
			Len  int    `json:"len" json-description:"the length of the quote"`
		}

		var collected []Args

		getQuote := tools.NewTool("get_quote",
			tools.WithDescription("get a quote from a character"),
			tools.WithArgSchema(Args{}),
			tools.WithFunction(func(ctx context.Context, call tools.Call) (string, error) {
				var a Args
				if err := json.Unmarshal(call.Argument, &a); err != nil {
					return "", err
				}
				collected = append(collected, a)
				return "", nil
			}),
		)

		res, err := g.
			System("You are a Shakespeare quote generator").
			SetTools(getQuote).
			Prompt(prompt.AsUser("Give me 3 quotes from different characters"))
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}

		if err := res.Eval(context.Background()); err != nil {
			t.Fatalf("Eval() error = %v", err)
		}

		if len(collected) != 3 {
			t.Fatalf("expected 3 tool calls, got %d", len(collected))
		}
	}
}

func testOutputSimple(g *gen.Generator) func(*testing.T) {
	return func(t *testing.T) {
		type Quote struct {
			CharacterName string `json:"character_name" json-enum:"Hamlet,Romeo,Juliet"`
			Quote         string `json:"quote"`
		}
		type Result struct {
			Quotes []Quote `json:"quotes"`
		}

		res, err := g.
			System("You are a Shakespeare quote generator").
			Output(schema.From(Result{})).
			StrictOutput(true).
			Prompt(prompt.AsUser("Give me 3 quotes from different characters"))
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}

		var result Result
		if err := res.Unmarshal(&result); err != nil {
			t.Fatalf("Unmarshal() error = %v", err)
		}

		if len(result.Quotes) != 3 {
			t.Fatalf("expected 3 quotes, got %d", len(result.Quotes))
		}

		allowed := []string{"Hamlet", "Romeo", "Juliet"}
		for _, q := range result.Quotes {
			if !slices.Contains(allowed, q.CharacterName) {
				t.Fatalf("expected character to be one of %v, got %q", allowed, q.CharacterName)
			}
		}
	}
}

func testStreamCount(g *gen.Generator) func(*testing.T) {
	return func(t *testing.T) {
		stream, err := g.Stream(prompt.AsUser("Count to ten, and spell it out. Example 'one, two, ... ten'"))
		if err != nil {
			t.Fatalf("Stream() error = %v", err)
		}

		var buf strings.Builder
		deltas := 0
		for r := range stream {
			switch r.Type {
			case gen.TYPE_DELTA:
				deltas++
				buf.WriteString(r.Content)
			case gen.TYPE_ERROR:
				t.Fatalf("stream error = %v", r.Content)
			}
		}

		if deltas == 0 {
			t.Fatalf("expected at least one TYPE_DELTA event, got zero")
		}

		low := strings.ToLower(buf.String())
		want := []string{"one", "two", "three", "four", "ten"}
		for _, w := range want {
			if !strings.Contains(low, w) {
				t.Fatalf("expected streamed text %q to contain %q", buf.String(), w)
			}
		}
	}
}

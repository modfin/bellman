package testsuite

import (
	"testing"

	"github.com/modfin/bellman/models/embed"
	"github.com/modfin/bellman/models/gen"
)

// Capabilities declares which gen-side features the model under test supports.
// The harness skips subtests whose flag is false so missing coverage surfaces
// in `go test -v` output instead of being silently elided.
type Capabilities struct {
	Tools            bool
	StructuredOutput bool
	Streaming        bool
	// Thinking flags whether this model supports extended thinking. When true,
	// agent tests turn on ThinkingBudget+IncludeThinkingParts so the
	// signed-turn replay path is exercised; when false, the same tests still
	// run end-to-end (multi-hop tool calls, BLOCK contract, structured
	// output) but skip thinking-specific assertions.
	Thinking            bool
	Agent               bool // agent.Run[T] — multi-depth tool-calling loop with structured result
	StreamThinkingTools bool // Stream() interleaving text + thinking + tool-call deltas (single turn) — implies Thinking
	StreamAgentMultiHop bool // Stream() driven multi-turn agent loop; signature replay assertions gated by Thinking
}

// EmbedCapabilities declares which embed-side features the model under test
// supports.
type EmbedCapabilities struct {
	Single   bool
	Many     bool
	Document bool
}

func Run(t *testing.T, g *gen.Generator, caps Capabilities) {
	t.Helper()

	model := g.Request.Model
	name := model.Name
	if name == "" {
		t.Fatal("model name is required")
	}

	t.Run(name, func(t *testing.T) {
		t.Run("hello", testHello(g))

		t.Run("tools", func(t *testing.T) {
			if !caps.Tools {
				t.Skip("capability Tools not advertised")
			}
			testTool(g)(t)
		})

		t.Run("output/simple", func(t *testing.T) {
			if !caps.StructuredOutput {
				t.Skip("capability StructuredOutput not advertised")
			}
			testOutputSimple(g)(t)
		})

		t.Run("stream/count", func(t *testing.T) {
			if !caps.Streaming {
				t.Skip("capability Streaming not advertised")
			}
			testStreamCount(g)(t)
		})

		t.Run("agent/run", func(t *testing.T) {
			if !caps.Agent {
				t.Skip("capability Agent not advertised")
			}
			testAgentRun(g, caps.Thinking)(t)
		})

		t.Run("agent/run_multihop", func(t *testing.T) {
			if !caps.Agent {
				t.Skip("capability Agent not advertised")
			}
			testAgentRunMultiHop(g, caps.Thinking)(t)
		})

		t.Run("stream/thinking_tools", func(t *testing.T) {
			if !caps.StreamThinkingTools {
				t.Skip("capability StreamThinkingTools not advertised")
			}
			testStreamThinkingTools(g)(t)
		})

		t.Run("stream/agent_multihop", func(t *testing.T) {
			if !caps.StreamAgentMultiHop {
				t.Skip("capability StreamAgentMultiHop not advertised")
			}
			testStreamAgentMultiHop(g, caps.Thinking)(t)
		})
	})
}

func RunEmbed(t *testing.T, e embed.Embeder, m embed.Model, caps EmbedCapabilities) {
	t.Helper()

	name := m.Name
	if name == "" {
		t.Fatal("model name is required")
	}

	t.Run(name, func(t *testing.T) {
		t.Run("embed/single", func(t *testing.T) {
			if !caps.Single {
				t.Skip("capability Single not advertised")
			}
			testEmbedSingle(e, m)(t)
		})

		t.Run("embed/many", func(t *testing.T) {
			if !caps.Many {
				t.Skip("capability Many not advertised")
			}
			testEmbedMany(e, m)(t)
		})

		t.Run("embed/document", func(t *testing.T) {
			if !caps.Document {
				t.Skip("capability Document not advertised")
			}
			testEmbedDocument(e, m)(t)
		})
	})
}

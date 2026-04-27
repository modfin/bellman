package testsuite

import (
	"testing"

	"github.com/modfin/bellman/models/embed"
	"github.com/modfin/bellman/models/gen"
)

const retryAttempts = 3

// Capabilities declares which gen-side features the model under test supports.
// The harness skips subtests whose flag is false so missing coverage surfaces
// in `go test -v` output instead of being silently elided.
type Capabilities struct {
	Tools               bool
	StructuredOutput    bool
	Streaming           bool
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
		t.Run("hello", func(t *testing.T) {
			withRetry(t, retryAttempts, testHello(g))
		})

		t.Run("tools", func(t *testing.T) {
			if !caps.Tools {
				t.Skip("capability Tools not advertised")
			}
			withRetry(t, retryAttempts, testTool(g))
		})

		t.Run("output/simple", func(t *testing.T) {
			if !caps.StructuredOutput {
				t.Skip("capability StructuredOutput not advertised")
			}
			withRetry(t, retryAttempts, testOutputSimple(g))
		})

		t.Run("stream/count", func(t *testing.T) {
			if !caps.Streaming {
				t.Skip("capability Streaming not advertised")
			}
			withRetry(t, retryAttempts, testStreamCount(g))
		})

		t.Run("agent/run", func(t *testing.T) {
			if !caps.Agent {
				t.Skip("capability Agent not advertised")
			}
			withRetry(t, retryAttempts, testAgentRun(g, caps.Thinking))
		})

		t.Run("agent/run_multihop", func(t *testing.T) {
			if !caps.Agent {
				t.Skip("capability Agent not advertised")
			}
			withRetry(t, retryAttempts, testAgentRunMultiHop(g, caps.Thinking))
		})

		t.Run("stream/thinking_tools", func(t *testing.T) {
			if !caps.StreamThinkingTools {
				t.Skip("capability StreamThinkingTools not advertised")
			}
			withRetry(t, retryAttempts, testStreamThinkingTools(g))
		})

		t.Run("stream/agent_multihop", func(t *testing.T) {
			if !caps.StreamAgentMultiHop {
				t.Skip("capability StreamAgentMultiHop not advertised")
			}
			withRetry(t, retryAttempts, testStreamAgentMultiHop(g, caps.Thinking))
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
			withRetry(t, retryAttempts, testEmbedSingle(e, m))
		})

		t.Run("embed/many", func(t *testing.T) {
			if !caps.Many {
				t.Skip("capability Many not advertised")
			}
			withRetry(t, retryAttempts, testEmbedMany(e, m))
		})

		t.Run("embed/document", func(t *testing.T) {
			if !caps.Document {
				t.Skip("capability Document not advertised")
			}
			withRetry(t, retryAttempts, testEmbedDocument(e, m))
		})
	})
}

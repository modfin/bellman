package testsuite

import (
	"fmt"
	"strings"

	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
)

func testPromptCache(g *gen.Generator) func(tester) {
	return func(t tester) {
		// Providers only cache prefixes above a model dependent minimum (1-2k
		// tokens), so the cacheable part has to be genuinely long. It also has
		// to be byte identical across both turns, since the prefix is the cache
		// key.
		var rules strings.Builder
		for i := 0; i < 300; i++ {
			rules.WriteString(fmt.Sprintf("Rule %d: when asked about topic %d, answer precisely and cite the rule number.\n", i, i))
		}

		cg := g.System("You answer questions about the following rule book.\n" + rules.String()).PromptCache(true)

		first, err := cg.Prompt(prompt.AsUser("Reply with the single word: one"))
		if err != nil {
			t.Fatalf("first Prompt() error = %v", err)
		}
		// A previous run may have left the prefix cached, so the first turn is
		// allowed to read it instead of writing it.
		if first.Metadata.CacheWriteTokens == 0 && first.Metadata.CachedTokens == 0 {
			t.Fatalf("first turn neither wrote nor read the prompt cache: %+v", first.Metadata)
		}

		second, err := cg.Prompt(prompt.AsUser("Reply with the single word: two"))
		if err != nil {
			t.Fatalf("second Prompt() error = %v", err)
		}
		if second.Metadata.CachedTokens == 0 {
			t.Fatalf("second turn did not read the prompt cache: %+v", second.Metadata)
		}
		if second.Metadata.CachedTokens > second.Metadata.InputTokens {
			t.Fatalf("CachedTokens %d exceeds InputTokens %d, cached tokens must be a subset of the input: %+v",
				second.Metadata.CachedTokens, second.Metadata.InputTokens, second.Metadata)
		}
	}
}

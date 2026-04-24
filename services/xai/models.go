package xai

import "github.com/modfin/bellman/models/gen"

const Provider = "xAI"

var GenModel_grok_4_20_reasoning = gen.Model{
	Provider:             Provider,
	Name:                 "grok-4.20-reasoning",
	UsesAdaptiveThinking: true,
}

var GenModel_grok_4_20_multi_agent = gen.Model{
	Provider:             Provider,
	Name:                 "grok-4.20-multi-agent",
	UsesAdaptiveThinking: true,
}

var GenModel_grok_4_1_fast_reasoning = gen.Model{
	Provider:             Provider,
	Name:                 "grok-4-1-fast-reasoning",
	UsesAdaptiveThinking: true,
}

var GenModel_grok_4 = gen.Model{
	Provider: Provider,
	Name:     "grok-4",
}

package vllm

import (
	"github.com/modfin/bellman/models/embed"
	"github.com/modfin/bellman/models/gen"
)

const Provider = "vLLM"

var EmbedModel_qwen_3_8b = embed.Model{
	Provider:         Provider,
	Name:             "Qwen/Qwen3-Embedding-8B",
	InputMaxTokens:   32_768,
	OutputDimensions: 4096,
}

var EmbedModel_qwen_3_4b = embed.Model{
	Provider:         Provider,
	Name:             "Qwen/Qwen3-Embedding-4B",
	InputMaxTokens:   32_768,
	OutputDimensions: 2560,
}

var GenModel_gpt_oss_20b = gen.Model{
	Provider: Provider,
	Name:     "openai/gpt-oss-20b",
}

package ollama

import (
	"github.com/modfin/bellman/models/embed"
	"github.com/modfin/bellman/models/gen"
)

const Provider = "Ollama"

var GenModel_llama_3_3 = gen.Model{
	Provider:    Provider,
	Name:        "llama3.3",
	Description: "New state of the art 70B model. Llama 3.3 70B offers similar performance compared to Llama 3.1 405B model.",
}

var GenModel_llama_3_2_vision_11b = gen.Model{
	Provider:    Provider,
	Name:        "llama3.2-vision:11b",
	Description: "Llama 3.2 Vision is a collection of instruction-tuned image reasoning generative models in 11B sizes.",
}

var GenModel_llama_3_2_vision_90b = gen.Model{
	Provider:    Provider,
	Name:        "llama3.2-vision:90b",
	Description: "Llama 3.2 Vision is a collection of instruction-tuned image reasoning generative models in 90B sizes.",
}

var GenModel_llama_3_2 = gen.Model{
	Provider:    Provider,
	Name:        "llama3.2",
	Description: "Meta's Llama 3.2 goes small with 3B models. alias for llama3.2:3b",
}

var GenModel_llama_3_2_3b = gen.Model{
	Provider:    Provider,
	Name:        "llama3.2:3b",
	Description: "Meta's Llama 3.2 goes small with 3B models.",
}
var GenModel_llama_3_2_1b = gen.Model{
	Provider:    Provider,
	Name:        "llama3.2:1b",
	Description: "Meta's Llama 3.2 goes small with 1B models.",
}
var GenModel_llama_3_1_8b = gen.Model{
	Provider:    Provider,
	Name:        "llama3.1:8b",
	Description: "Llama 3.1 is a new state-of-the-art model from Meta available in 8Bparameter sizes.",
}
var GenModel_llama_3_1_70b = gen.Model{
	Provider:    Provider,
	Name:        "llama3.1:70b",
	Description: "Llama 3.1 is a new state-of-the-art model from Meta available in 70B parameter sizes.",
}
var GenModel_llama_3_1_405b = gen.Model{
	Provider:    Provider,
	Name:        "llama3.1:405b",
	Description: "Llama 3.1 is a new state-of-the-art model from Meta available in 405B parameter sizes.",
}

var GenModel_gemma2 = gen.Model{
	Provider:    Provider,
	Name:        "gemma2",
	Description: "Google Gemma 2 is a high-performing and efficient model available in three sizes: 9B",
}

var GenModel_gemma2_9b = gen.Model{
	Provider:    Provider,
	Name:        "gemma2:9b",
	Description: "Google Gemma 2 is a high-performing and efficient model available in three sizes: 2B, 9B, and 27B.",
}
var GenModel_gemma2_2b = gen.Model{
	Provider:    Provider,
	Name:        "gemma2:2b",
	Description: "Google Gemma 2 is a high-performing and efficient model available in three sizes: 2B, 9B, and 27B.",
}
var GenModel_gemma2_27b = gen.Model{
	Provider:    Provider,
	Name:        "gemma2:27b",
	Description: "Google Gemma 2 is a high-performing and efficient model available in three sizes: 2B, 9B, and 27B.",
}

// https://platform.openai.com/docs/models#embeddings

var EmbedModel_nomic_embed_text = embed.Model{
	Provider:         Provider,
	Name:             "nomic-embed-text",
	Description:      "Most capable embedding Model for both english and non-english tasks, https://huggingface.co/nomic-ai/nomic-embed-text-v1.5",
	InputMaxTokens:   2048,
	OutputDimensions: 768,

	// nomic-bert.context_length 2048
	// nomic-bert.embedding_length 768
	// nomic-bert.feed_forward_length 3072
}

var EmbedModel_mxbai_embed_large = embed.Model{
	Provider:         Provider,
	Name:             "mxbai-embed-large",
	Description:      "State-of-the-art large embedding model from mixedbread.ai, https://huggingface.co/mixedbread-ai/mxbai-embed-large-v1",
	InputMaxTokens:   512,
	OutputDimensions: 1024,
	// bert.context_length 512
	// bert.embedding_length 1024
	// bert.feed_forward_length 4096
}

var EmbedModel_paraphrase_multilingual = embed.Model{
	Provider:         Provider,
	Name:             "paraphrase-multilingual",
	Description:      "Sentence-transformers model that can be used for tasks like clustering or semantic search., https://ollama.com/library/paraphrase-multilingual",
	InputMaxTokens:   512,
	OutputDimensions: 768,

	//bert.context_length 512
	//bert.embedding_length 768
	//bert.feed_forward_length 3072
}

var EmbedModel_bge_large = embed.Model{
	Provider:         Provider,
	Name:             "bge-large",
	Description:      "Embedding model from BAAI mapping texts to vectors, https://ollama.com/library/bge-large",
	InputMaxTokens:   512,
	OutputDimensions: 1024,

	//bert.context_length 512
	//bert.embedding_length 1024
	//bert.feed_forward_length 4096
}

var EmbedModel_bge_m3 = embed.Model{
	Provider:         Provider,
	Name:             "bge-m3",
	Description:      "GE-M3 is a new model from BAAI distinguished for its versatility in Multi-Functionality, Multi-Linguality, and Multi-Granularity. https://ollama.com/library/bge-m3",
	InputMaxTokens:   8192,
	OutputDimensions: 1024,

	//bert.context_length 8192
	//bert.embedding_length 1024
	//bert.feed_forward_length 4096
}
var EmbedModel_qwen3_06b = embed.Model{
	Provider:         Provider,
	Name:             "qwen3-embedding:0.6b",
	Description:      "Building upon the foundational models of the Qwen3 series, Qwen3 Embedding provides a comprehensive range of text embeddings models in various sizes. 0.6B. https://ollama.com/library/qwen3-embedding",
	InputMaxTokens:   32_000,
	OutputDimensions: 4096,
}
var EmbedModel_qwen3_4b = embed.Model{
	Provider:         Provider,
	Name:             "qwen3-embedding:4b",
	Description:      "Building upon the foundational models of the Qwen3 series, Qwen3 Embedding provides a comprehensive range of text embeddings models in various sizes. 4B. https://ollama.com/library/qwen3-embedding",
	InputMaxTokens:   32_000,
	OutputDimensions: 4096,
}
var EmbedModel_qwen3_8b = embed.Model{
	Provider:         Provider,
	Name:             "qwen3-embedding:8b",
	Description:      "Building upon the foundational models of the Qwen3 series, Qwen3 Embedding provides a comprehensive range of text embeddings models in various sizes. 8B. https://ollama.com/library/qwen3-embedding",
	InputMaxTokens:   32_000,
	OutputDimensions: 4096,
}

var EmbedModels = map[string]embed.Model{
	EmbedModel_nomic_embed_text.Name:        EmbedModel_nomic_embed_text,
	EmbedModel_mxbai_embed_large.Name:       EmbedModel_mxbai_embed_large,
	EmbedModel_paraphrase_multilingual.Name: EmbedModel_paraphrase_multilingual,
	EmbedModel_bge_large.Name:               EmbedModel_bge_large,
	EmbedModel_bge_m3.Name:                  EmbedModel_bge_m3,
}

var GenModels = map[string]gen.Model{
	GenModel_llama_3_3.Name:            GenModel_llama_3_3,
	GenModel_llama_3_2_vision_11b.Name: GenModel_llama_3_2_vision_11b,
	GenModel_llama_3_2_vision_90b.Name: GenModel_llama_3_2_vision_90b,
	GenModel_llama_3_2_3b.Name:         GenModel_llama_3_2_3b,
	GenModel_llama_3_2_1b.Name:         GenModel_llama_3_2_1b,
	GenModel_llama_3_1_8b.Name:         GenModel_llama_3_1_8b,
	GenModel_llama_3_1_70b.Name:        GenModel_llama_3_1_70b,
	GenModel_llama_3_1_405b.Name:       GenModel_llama_3_1_405b,
}

package voyageai

import (
	"github.com/modfin/bellman/models/embed"
)

const Provider = "VoyageAI"

// https://docs.voyageai.com/docs/embeddings
var EmbedModel_voyage_3 = embed.Model{
	Provider:         Provider,
	Name:             "voyage-3",
	InputMaxTokens:   32000,
	OutputDimensions: 1024,
	Description:      "Optimized for general-purpose and multilingual retrieval quality.",
}

var EmbedModel_voyage_3_lite = embed.Model{
	Provider:         Provider,
	Name:             "voyage-3-lite",
	InputMaxTokens:   32000,
	OutputDimensions: 512,
	Description:      "Optimized for latency and cost",
}

var EmbedModel_voyage_finance_2 = embed.Model{
	Provider:         Provider,
	Name:             "voyage-finance-2",
	InputMaxTokens:   32000,
	OutputDimensions: 1024,
	Description:      "Optimized for finance retrieval and RAG.",
}

var EmbedModel_voyage_multilingual_2 = embed.Model{
	Provider:         Provider,
	Name:             "voyage-multilingual-2",
	InputMaxTokens:   32000,
	OutputDimensions: 1024,
	Description:      "Optimized for multilingual retrieval and RAG.",
}

var EmbedModel_voyage_law_2 = embed.Model{
	Provider:         Provider,
	Name:             "voyage-law-2",
	InputMaxTokens:   16000,
	OutputDimensions: 1024,
	Description:      "Optimized for legal and long-context retrieval and RAG. Also improved performance across all domains.",
}

var EmbedModel_voyage_code_2 = embed.Model{
	Provider:         Provider,
	Name:             "voyage-code-2",
	InputMaxTokens:   16000,
	OutputDimensions: 1536,
	Description:      "Optimized for code retrieval (17% better than alternatives)",
}

var EmbedModel_voyage_large_2_instruct = embed.Model{
	Provider:         Provider,
	Name:             "voyage-large-2-instruct",
	InputMaxTokens:   16000,
	OutputDimensions: 1024,
	Description:      "Top of MTEB leaderboard . Instruction-tuned general-purpose embedding model optimized for clustering, classification, and retrieval. For retrieval, please use input_type parameter to specify whether the text is a query or document. For classification and clustering, please use the instructions here . See blog post for details. We recommend existing voyage-large-2-instruct users to transition to voyage-3",
}

var EmbedModel_voyage_large_2 = embed.Model{
	Provider:         Provider,
	Name:             "voyage-large-2",
	InputMaxTokens:   16000,
	OutputDimensions: 1536,
	Description:      "General-purpose embedding model that is optimized for retrieval quality (e.g., better than OpenAI V3 Large). Please transition to voyage-3.",
}

var EmbedModel_voyage_2 = embed.Model{
	Provider:         Provider,
	Name:             "voyage-2",
	InputMaxTokens:   4000,
	OutputDimensions: 1024,
	Description:      "General-purpose embedding model optimized for a balance between cost, latency, and retrieval quality. Please transition to voyage-3-lite.",
}

var EmbedModels = map[string]embed.Model{
	EmbedModel_voyage_3.Name:              EmbedModel_voyage_3,
	EmbedModel_voyage_3_lite.Name:         EmbedModel_voyage_3_lite,
	EmbedModel_voyage_finance_2.Name:      EmbedModel_voyage_finance_2,
	EmbedModel_voyage_multilingual_2.Name: EmbedModel_voyage_multilingual_2,
	EmbedModel_voyage_law_2.Name:          EmbedModel_voyage_law_2,
	EmbedModel_voyage_code_2.Name:         EmbedModel_voyage_code_2,

	EmbedModel_voyage_large_2_instruct.Name: EmbedModel_voyage_large_2_instruct,
	EmbedModel_voyage_large_2.Name:          EmbedModel_voyage_large_2,
	EmbedModel_voyage_2.Name:                EmbedModel_voyage_2,
}

package vertexai

import (
	"github.com/modfin/bellman/models/embed"
	"github.com/modfin/bellman/models/gen"
)

// https://cloud.google.com/vertex-ai/generative-ai/docs/learn/models#gemini-models

const Provider = "VertexAI"

var GenModel_gemini_2_5_pro_latest = gen.Model{
	Provider:       Provider,
	Name:           "gemini-2.5-pro",
	InputMaxToken:  1_048_576,
	OutputMaxToken: 65_536,
}
var GenModel_gemini_2_5_flash_latest = gen.Model{
	Provider:       Provider,
	Name:           "gemini-2.5-flash",
	InputMaxToken:  1_048_576,
	OutputMaxToken: 65_536,
}

var GenModel_gemini_2_5_flash_lite_latest = gen.Model{
	Provider:       Provider,
	Name:           "gemini-2.5-flash-lite",
	InputMaxToken:  1_048_576,
	OutputMaxToken: 65_536,
}

// https://cloud.google.com/vertex-ai/generative-ai/docs/embeddings/get-text-embeddings#supported-models

var EmbedModel_gemini_001 = embed.Model{
	Provider:         Provider,
	Name:             "gemini-embedding-001",
	Description:      "State-of-the-art performance across English, multilingual and code tasks. It unifies the previously specialized models like text-embedding-005 and text-multilingual-embedding-002 and achieves better performance in their respective domains.",
	InputMaxTokens:   2048,
	OutputDimensions: 3072,
}
var EmbedModel_text_005 = embed.Model{
	Provider:         Provider,
	Name:             "text-embedding-005",
	Description:      "see https://cloud.google.com/vertex-ai/generative-ai/docs/model-reference/text-embeddings-api",
	InputMaxTokens:   2048,
	OutputDimensions: 768,
}
var EmbedModel_text_004 = embed.Model{
	Provider:         Provider,
	Name:             "text-embedding-004",
	Description:      "see https://cloud.google.com/vertex-ai/generative-ai/docs/model-reference/text-embeddings-api",
	InputMaxTokens:   2048,
	OutputDimensions: 768,
}
var EmbedMode_multilang_002 = embed.Model{
	Provider:         Provider,
	Name:             "text-multilingual-embedding-002",
	Description:      "see https://cloud.google.com/vertex-ai/generative-ai/docs/model-reference/text-embeddings-api",
	InputMaxTokens:   2048,
	OutputDimensions: 768,
}

// var EmbedModel_text_gecko_001 = embed.Model{  // deprecated?
//
//		Provider:         Provider,
//		Name:             "textembedding-gecko@001",
//		Description:      "see https://cloud.google.com/vertex-ai/generative-ai/docs/model-reference/text-embeddings-api",
//		InputMaxTokens:   2048,
//		OutputDimensions: 768,
//	}
var EmbedModel_text_gecko_003 = embed.Model{
	Provider:         Provider,
	Name:             "textembedding-gecko@003",
	Description:      "see https://cloud.google.com/vertex-ai/generative-ai/docs/model-reference/text-embeddings-api",
	InputMaxTokens:   2048,
	OutputDimensions: 768,
}

var EmbedModel_text_gecko_multilang_001 = embed.Model{
	Provider:         Provider,
	Name:             "textembedding-gecko-multilingual@001",
	Description:      "see https://cloud.google.com/vertex-ai/generative-ai/docs/model-reference/text-embeddings-api",
	InputMaxTokens:   2048,
	OutputDimensions: 768,
}

const EmbedDimensions = 768

const TypeDocument embed.Type = "RETRIEVAL_DOCUMENT"
const TypeQuery embed.Type = "RETRIEVAL_QUERY"
const TypeQuestionAnswer embed.Type = "QUESTION_ANSWERING"
const TypeFactVerification embed.Type = "FACT_VERIFICATION"
const TypeCodeRetrieval embed.Type = "CODE_RETRIEVAL_QUERY"
const TypeClustering embed.Type = "CLUSTERING"
const TypeClassification embed.Type = "CLASSIFICATION"
const TypeSemanticSimilarity embed.Type = "SEMANTIC_SIMILARITY"

var EmbedModels = map[string]embed.Model{
	EmbedModel_text_005.Name:     EmbedModel_text_005,
	EmbedModel_text_004.Name:     EmbedModel_text_004,
	EmbedMode_multilang_002.Name: EmbedMode_multilang_002,
	EmbedModel_gemini_001.Name:   EmbedModel_gemini_001,
}

var GenModels = map[string]gen.Model{
	GenModel_gemini_2_5_pro_latest.Name:   GenModel_gemini_2_5_pro_latest,
	GenModel_gemini_2_5_flash_latest.Name: GenModel_gemini_2_5_flash_latest,
}

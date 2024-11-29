package vertexai

import "github.com/modfin/bellman"

// https://cloud.google.com/vertex-ai/generative-ai/docs/learn/models#gemini-models
//type GenModel string

var GenModel_gemini_Experiment_114 = bellman.GenModel{
	Name:                    "gemini-exp-1114",
	Description:             "",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}

var GenModel_gemini_1_5_flash = bellman.GenModel{
	Name:                    "gemini-1.5-flash",
	Description:             "Fast and versatile performance across a diverse variety of tasks",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_gemini_1_5_flash_001 = bellman.GenModel{
	Name:                    "gemini-1.5-flash-002",
	Description:             "Fast and versatile performance across a diverse variety of tasks",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_gemini_1_5_flash_002 = bellman.GenModel{
	Name:                    "gemini-1.5-flash-001",
	Description:             "Fast and versatile performance across a diverse variety of tasks",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}

//var GenModel_gemini_1_5_flash_8b_latest = bellman.GenModel{Name: "gemini-1.5-flash-8b-latest",}

var GenModel_gemini_1_5_flash_8b = bellman.GenModel{
	Name:                    "gemini-1.5-flash-8b",
	Description:             "High volume and lower intelligence tasks",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_gemini_1_5_flash_8b_001 = bellman.GenModel{
	Name:                    "gemini-1.5-flash-8b-001",
	Description:             "High volume and lower intelligence tasks",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}

//var GenModel_gemini_1_5_pro_latest = bellman.GenModel{Name: "gemini-1.5-pro-latest",}

var GenModel_gemini_1_5_pro = bellman.GenModel{
	Name:                    "gemini-1.5-pro",
	Description:             "Complex reasoning tasks requiring more intelligence",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_gemini_1_5_pro_002 = bellman.GenModel{
	Name:                    "gemini-1.5-pro-002",
	Description:             "Complex reasoning tasks requiring more intelligence",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_gemini_1_5_pro_001 = bellman.GenModel{
	Name:                    "gemini-1.5-pro-001",
	Description:             "Complex reasoning tasks requiring more intelligence",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}

// https://cloud.google.com/vertex-ai/generative-ai/docs/embeddings/get-text-embeddings#supported-models

var EmbedModel_text_005 = bellman.EmbedModel{
	Name:             "text-embedding-005",
	Description:      "see https://cloud.google.com/vertex-ai/generative-ai/docs/model-reference/text-embeddings-api",
	InputMaxTokens:   2048,
	OutputDimensions: 768,
}
var EmbedModel_text_004 = bellman.EmbedModel{
	Name:             "text-embedding-004",
	Description:      "see https://cloud.google.com/vertex-ai/generative-ai/docs/model-reference/text-embeddings-api",
	InputMaxTokens:   2048,
	OutputDimensions: 768,
}
var EmbedMode_multilang_002 = bellman.EmbedModel{
	Name:             "text-multilingual-embedding-002",
	Description:      "see https://cloud.google.com/vertex-ai/generative-ai/docs/model-reference/text-embeddings-api",
	InputMaxTokens:   2048,
	OutputDimensions: 768,
}
var EmbedModel_text_gecko_001 = bellman.EmbedModel{
	Name:             "textembedding-gecko@001",
	Description:      "see https://cloud.google.com/vertex-ai/generative-ai/docs/model-reference/text-embeddings-api",
	InputMaxTokens:   2048,
	OutputDimensions: 768,
}
var EmbedModel_text_gecko_003 = bellman.EmbedModel{
	Name:             "textembedding-gecko@003",
	Description:      "see https://cloud.google.com/vertex-ai/generative-ai/docs/model-reference/text-embeddings-api",
	InputMaxTokens:   2048,
	OutputDimensions: 768,
}
var EmbedModel_text_gecko_multilang_001 = bellman.EmbedModel{
	Name:             "textembedding-gecko-multilingual@001",
	Description:      "see https://cloud.google.com/vertex-ai/generative-ai/docs/model-reference/text-embeddings-api",
	InputMaxTokens:   2048,
	OutputDimensions: 768,
}

const EmbedDimensions = 768

type EmbedType string

const EmbedTypeQuery EmbedType = "RETRIEVAL_QUERY"
const EmbedTypeDocument EmbedType = "RETRIEVAL_DOCUMENT"
const EmbedTypeSimilarity EmbedType = "SEMANTIC_SIMILARITY"
const EmbedTypeClassification EmbedType = "CLASSIFICATION"
const EmbedTypeClustring EmbedType = "CLUSTERING"
const EmbedTypeQA EmbedType = "QUESTION_ANSWERING"
const EmbedTypeVerification EmbedType = "FACT_VERIFICATION"
const EmbedTypeCode EmbedType = "CODE_RETRIEVAL_QUERY"

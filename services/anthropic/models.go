package anthropic

import (
	"github.com/modfin/bellman/models/gen"
)

const Provider = "Anthropic"

const Version = "2023-06-01"

//curl https://api.anthropic.com/v1/models/claude-3-5-sonnet-20241022 \
//  --header "x-api-key: $ANTHROPIC_API_KEY" \
//  --header "anthropic-version: 2023-06-01"
//{
//  "data": [
//    {
//      "type": "model",
//      "id": "claude-3-5-sonnet-20241022",
//      "display_name": "Claude 3.5 Sonnet (New)",
//      "created_at": "2024-10-22T00:00:00Z"
//    },
//	{...

//type GenModel string

// https://docs.anthropic.com/en/docs/about-claude/models
var GenModel_3_7_sonnet_latest = gen.Model{
	Provider:       Provider,
	Name:           "claude-3-7-sonnet-latest",
	InputMaxToken:  200_000,
	OutputMaxToken: 64_000,
}
var GenModel_3_7_sonnet_20250219 = gen.Model{
	Provider:       Provider,
	Name:           "claude-3-7-sonnet-20250219",
	InputMaxToken:  200_000,
	OutputMaxToken: 64_000,
}
var GenModel_4_0_sonnet_20250514 = gen.Model{
	Provider:       Provider,
	Name:           "claude-sonnet-4-20250514",
	InputMaxToken:  200_000,
	OutputMaxToken: 64_000,
}
var GenModel_4_5_sonnet_latest = gen.Model{
	Provider:       Provider,
	Description:    "Our smartest model for complex agents and coding",
	Name:           "claude-4-5-sonnet",
	InputMaxToken:  200_000,
	OutputMaxToken: 64_000,
}
var GenModel_4_5_sonnet_20250929 = gen.Model{
	Provider:       Provider,
	Description:    "Our smartest model for complex agents and coding",
	Name:           "claude-sonnet-4-5-20250929",
	InputMaxToken:  200_000,
	OutputMaxToken: 64_000,
}
var GenModel_3_5_sonnet_latest = gen.Model{
	Provider:                Provider,
	Name:                    "claude-3-5-sonnet-latest",
	Description:             "",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_3_5_sonnet_20241022 = gen.Model{
	Provider:                Provider,
	Name:                    "claude-3-5-sonnet-20241022",
	Description:             "",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_3_5_sonnet_20240620 = gen.Model{
	Provider:                Provider,
	Name:                    "claude-3-5-sonnet-20240620",
	Description:             "",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}

//var GenModel_3_sonnet_20240229 = gen.Model{
//	Provider:                Provider,
//	Name:                    "claude-3-sonnet-20240229",
//	Description:             "",
//	InputContentTypes:       nil,
//	InputMaxToken:           0,
//	OutputMaxToken:          0,
//	SupportTools:            false,
//	SupportStructuredOutput: false,
//}

var GenModel_4_5_haiku_latest = gen.Model{
	Provider:                Provider,
	Name:                    "claude-haiku-4-5",
	Description:             "Our fastest model with near-frontier intelligence",
	InputContentTypes:       nil,
	InputMaxToken:           200_000,
	OutputMaxToken:          64_000,
	SupportTools:            true,
	SupportStructuredOutput: true,
}
var GenModel_4_5_haiku_20251001 = gen.Model{
	Provider:                Provider,
	Name:                    "claude-haiku-4-5-20251001",
	Description:             "Our fastest model with near-frontier intelligence",
	InputContentTypes:       nil,
	InputMaxToken:           200_000,
	OutputMaxToken:          64_000,
	SupportTools:            true,
	SupportStructuredOutput: true,
}

var GenModel_3_5_haiku_latest = gen.Model{
	Provider:                Provider,
	Name:                    "claude-3-5-haiku-latest",
	Description:             "",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_3_5_haiku_20241022 = gen.Model{
	Provider:                Provider,
	Name:                    "claude-3-5-haiku-20241022",
	Description:             "",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_3_haiku_20240307 = gen.Model{
	Provider:                Provider,
	Name:                    "claude-3-haiku-20240307",
	Description:             "",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}

var GenModel_3_opus_latest = gen.Model{
	Provider:                Provider,
	Name:                    "claude-3-opus-latest",
	Description:             "",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_3_opus_20240229 = gen.Model{
	Provider:                Provider,
	Name:                    "claude-3-opus-20240229",
	Description:             "",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_4_0_opus_20250514 = gen.Model{
	Provider:                Provider,
	Name:                    "claude-opus-4-20250514",
	Description:             "",
	InputContentTypes:       nil,
	InputMaxToken:           0,
	OutputMaxToken:          0,
	SupportTools:            false,
	SupportStructuredOutput: false,
}
var GenModel_4_1_opus_20250805 = gen.Model{
	Provider:                Provider,
	Name:                    "claude-opus-4-1-20250805",
	Description:             "Exceptional model for specialized reasoning tasks",
	InputContentTypes:       nil,
	InputMaxToken:           200_000,
	OutputMaxToken:          32_000,
	SupportTools:            false,
	SupportStructuredOutput: false,
}

var GenModels = map[string]gen.Model{
	GenModel_3_5_sonnet_latest.Name:   GenModel_3_5_sonnet_latest,
	GenModel_3_5_sonnet_20241022.Name: GenModel_3_5_sonnet_20241022,
	//GenModel_3_5_sonnet_20240620.Name: GenModel_3_5_sonnet_20240620,
	GenModel_3_5_haiku_latest.Name:   GenModel_3_5_haiku_latest,
	GenModel_3_5_haiku_20241022.Name: GenModel_3_5_haiku_20241022,
	//GenModel_3_haiku_20240307.Name:    GenModel_3_haiku_20240307,
	GenModel_3_opus_latest.Name:   GenModel_3_opus_latest,
	GenModel_3_opus_20240229.Name: GenModel_3_opus_20240229,
}

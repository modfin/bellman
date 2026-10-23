package main

import (
	"encoding/json"
	"fmt"

	"github.com/modfin/bellman/models/gen"
)

// legacyGenRequest accepts the thinking_budget field sent by bellman clients from before
// thinking_level replaced it.
type legacyGenRequest struct {
	gen.FullRequest
	ThinkingBudget *int `json:"thinking_budget,omitempty"`
}

func decodeGenRequest(body []byte) (gen.FullRequest, error) {
	var req legacyGenRequest
	err := json.Unmarshal(body, &req)
	if err != nil {
		return gen.FullRequest{}, err
	}
	if req.ThinkingLevel == "" && req.ThinkingBudget != nil {
		req.ThinkingLevel = budgetToThinkingLevel(*req.ThinkingBudget)
	}
	if req.ThinkingLevel != "" && !req.ThinkingLevel.Valid() {
		return gen.FullRequest{}, fmt.Errorf("unknown thinking_level %q", req.ThinkingLevel)
	}
	return req.FullRequest, nil
}

// budgetToThinkingLevel uses the thresholds bellman used to translate budgets into reasoning effort.
// A negative budget meant dynamic thinking, which is the model default.
func budgetToThinkingLevel(budget int) gen.ThinkingLevel {
	switch {
	case budget < 0:
		return ""
	case budget == 0:
		return gen.ThinkingLevelNone
	case budget < 2_000:
		return gen.ThinkingLevelLow
	case budget < 10_000:
		return gen.ThinkingLevelMedium
	default:
		return gen.ThinkingLevelHigh
	}
}

package utils

import (
	"context"
	"encoding/json"
	"regexp"

	"github.com/modfin/bellman/schema"
	"github.com/modfin/bellman/tools"
)

// Regex to find invalid characters (only letters, numbers, underscores, dashes allowed)
var invalidNameChars = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

func ParseJsonSchemaTools(rawTools []interface{}, enablePTC bool) []tools.Tool {
	var parsedTools []tools.Tool

	for _, rt := range rawTools {
		jsonBytes, _ := json.Marshal(rt)

		var tDef struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			Parameters  json.RawMessage `json:"parameters"`
			Response    json.RawMessage `json:"response"`
		}

		// Handle BFCL's nested "function" wrapper if present
		var wrapper struct {
			Function json.RawMessage `json:"function"`
		}
		if err := json.Unmarshal(jsonBytes, &wrapper); err == nil && len(wrapper.Function) > 0 {
			_ = json.Unmarshal(wrapper.Function, &tDef)
		} else {
			_ = json.Unmarshal(jsonBytes, &tDef)
		}

		if tDef.Name == "" {
			continue
		}

		// Some Toolman models rejects dots. "math.factorial" -> "math_factorial"
		sanitizedName := invalidNameChars.ReplaceAllString(tDef.Name, "_") // TODO: check bench compatability

		// convert raw JSON parameters to Toolman-compatible JSON schema
		paramSchema := parseSchemaRawToJSON(tDef.Parameters)
		responseSchema := parseSchemaRawToJSON(tDef.Response)
		normalizeBFCLSchema(&paramSchema, false)
		normalizeBFCLSchema(&responseSchema, true)

		tool := tools.NewTool(sanitizedName,
			tools.WithDescription(tDef.Description),
			tools.WithPTC(enablePTC),
			tools.WithFunction(
				func(context.Context, tools.Call) (string, error) { return "{}", nil },
			),
		)

		tool.ArgumentSchema = &paramSchema
		tool.ResponseSchema = &responseSchema // TODO: cant use since we cant inject real response from BFCL!!!!!!

		parsedTools = append(parsedTools, tool)
	}

	return parsedTools
}

// parseSchemaRawToJSON converts raw JSON parameters to Toolman-compatible JSON schema
func parseSchemaRawToJSON(Parameters json.RawMessage) schema.JSON {
	// "dict" -> "object"
	var paramSchema schema.JSON

	if len(Parameters) > 0 {
		var check map[string]interface{}
		if err := json.Unmarshal(Parameters, &check); err == nil {

			typeVal, _ := check["type"].(string)

			// CFB uses "dict", Toolman wants "object"
			if typeVal == "dict" {
				check["type"] = "object"
				typeVal = "object" // Update for the check below
			}

			// If type is NOT object (e.g. "string"), must wrap it
			if typeVal != "" && typeVal != "object" {
				wrapped := map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"arg": check, // Wrap original schema
					},
					"required": []string{"arg"},
				}
				fixedBytes, _ := json.Marshal(wrapped)
				_ = json.Unmarshal(fixedBytes, &paramSchema)
			} else {
				// It's a valid object/dict, but we might have modified "type" in check
				// So we marshal 'check' back, not 'tDef.Parameters'
				fixedBytes, _ := json.Marshal(check)
				_ = json.Unmarshal(fixedBytes, &paramSchema)
			}
		}
	} else {
		// Handle empty parameters
		emptyObj := map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
		b, _ := json.Marshal(emptyObj)
		_ = json.Unmarshal(b, &paramSchema)
	}

	return paramSchema
}

// normalizeBFCLSchema recursively cleans non-standard types from BFCL datasets
func normalizeBFCLSchema(s *schema.JSON, require bool) { // Replace *schema.JSON with your actual struct type if different
	if s == nil {
		return
	}

	// Fix the Pythonic/BFCL/CFB type dialects (JSON)
	switch s.Type {
	case "dict":
		s.Type = "object"
	case "list":
		s.Type = "array"
	case "int":
		s.Type = "integer"
	case "float":
		s.Type = "number"
	case "bool":
		s.Type = "boolean"
	}

	// if response --> set all fields to required
	if require && s.Type == "object" && len(s.Properties) > 0 && len(s.Required) == 0 {
		for key := range s.Properties {
			s.Required = append(s.Required, key)
		}
	}

	// Recursively traverse and fix nested properties (for objects)
	for _, prop := range s.Properties {
		normalizeBFCLSchema(prop, require)
	}

	// Recursively traverse and fix array items (for lists/arrays)
	if s.Items != nil {
		normalizeBFCLSchema(s.Items, require)
	}
}

package openai

import (
	"github.com/modfin/bellman/schema"
	"strconv"
)

// https://platform.openai.com/docs/guides/structured-outputs#supported-schemas

type DataType string

const (
	Object  DataType = "object"
	Number  DataType = "number"
	Integer DataType = "integer"
	String  DataType = "string"
	Array   DataType = "array"
	Null    DataType = "null"
	Boolean DataType = "boolean"
)

type JSONSchema struct {
	// Type specifies the data type of the schema. OpenAI uses []string{Type, Null} to represent nullable types.
	Type any `json:"type,omitempty"`
	// Description is the description of the schema.
	Description string `json:"description,omitempty"`
	// Enum is used to restrict a value to a fixed set of values. It must be an array with at least
	// one element, where each element is unique. You will probably only use this with strings.
	Enum []string `json:"enum,omitempty"`
	// Properties describes the properties of an object, if the schema type is Object.
	Properties map[string]JSONSchema `json:"properties,omitempty"`
	// Required specifies which properties are required, if the schema type is Object.
	Required []string `json:"required,omitempty"`
	// Items specifies which data type an array contains, if the schema type is Array.
	Items *JSONSchema `json:"items,omitempty"`
	// AdditionalProperties is used to control the handling of properties in an object
	// that are not explicitly defined in the properties section of the schema. example:
	// additionalProperties: true
	// additionalProperties: false
	// additionalProperties: jsonschema.JSONSchema{Type: jsonschema.String}
	AdditionalProperties any `json:"additionalProperties,omitempty"`
}

func fromBellmanSchema(bellmanSchema *schema.JSON) *JSONSchema {
	def := &JSONSchema{
		Description: bellmanSchema.Description,
		Required:    bellmanSchema.Required,
	}
	switch bellmanSchema.Type {
	case schema.Object:
		def.Type = Object
	case schema.Array:
		def.Type = Array
	case schema.String:
		def.Type = String
	case schema.Number:
		def.Type = Number
	case schema.Integer:
		def.Type = Integer
	case schema.Boolean:
		def.Type = Boolean
	default:
		def.Type = String
	}

	if len(bellmanSchema.Properties) > 0 {
		def.Properties = make(map[string]JSONSchema)
		for key, prop := range bellmanSchema.Properties {
			def.Properties[key] = *fromBellmanSchema(prop)
		}
	}
	if bellmanSchema.Items != nil {
		def.Items = fromBellmanSchema(bellmanSchema.Items)
	}

	if bellmanSchema.AdditionalProperties != nil {
		def.AdditionalProperties = *fromBellmanSchema(bellmanSchema.AdditionalProperties)
	}

	if bellmanSchema.Nullable {
		def.Type = []any{def.Type, Null}
	}

	if len(bellmanSchema.Enum) > 0 {
		def.Enum = make([]string, len(bellmanSchema.Enum))
		for i, e := range bellmanSchema.Enum {
			switch e.(type) {
			case string:
				def.Enum[i] = e.(string)
			case bool:
				def.Enum[i] = strconv.FormatBool(e.(bool))
			case int, int32, int64:
				def.Enum[i] = strconv.FormatInt(e.(int64), 10)
			case float32, float64:
				def.Enum[i] = strconv.FormatFloat(e.(float64), 'f', -1, 64)
			}
		}
	}
	return def
}

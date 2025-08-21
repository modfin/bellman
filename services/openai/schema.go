package openai

import (
	"github.com/modfin/bellman/schema"
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
	Ref  string                 `json:"$ref,omitempty"`  // #/$defs/... etc, overrides everything else
	Defs map[string]*JSONSchema `json:"$defs,omitempty"` // for $ref
	// Type specifies the data type of the schema. OpenAI uses []string{Type, Null} to represent nullable types.
	Type any `json:"type,omitempty"`
	// Description is the description of the schema.
	Description string `json:"description,omitempty"`
	// Enum is used to restrict a value to a fixed set of values. It must be an array with at least
	// one element, where each element is unique. You will probably only use this with strings.
	Enum    []any  `json:"enum,omitempty"`
	Pattern string `json:"pattern,omitempty"` // Regular expression that the string must match.
	Format  string `json:"format,omitempty"`  // Format of the data, e.g. "email", "date-time", etc.
	// Properties describes the properties of an object, if the schema type is Object.
	Properties *map[string]JSONSchema `json:"properties,omitempty"`
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

	MinLength int     `json:"minLength,omitempty"` // Minimum length of the string.
	MaxLength int     `json:"maxLength,omitempty"` // Maximum length of the string.
	Minimum   float64 `json:"minimum,omitempty"`   // Minimum value of the integer and number types.
	Maximum   float64 `json:"maximum,omitempty"`   // Minimum value of the integer and number types.
	MinItems  int     `json:"minItems,omitempty"`  // Minimum number of items in an array.
	MaxItems  int     `json:"maxItems,omitempty"`  // Maximum number of items in an array.
}

func (s JSONSchema) IsObjectRequired() bool {
	if s.Type == Object && s.Properties != nil && len(*s.Properties) == 0 {
		return false
	}
	return true
}
func newNilMap() *map[string]JSONSchema {
	m := make(map[string]JSONSchema)
	return &m
}

func fromBellmanSchema(bellmanSchema *schema.JSON) *JSONSchema {
	if bellmanSchema.Ref != "" {
		return &JSONSchema{
			Ref: bellmanSchema.Ref,
		}
	}
	def := &JSONSchema{
		Description:          bellmanSchema.Description,
		Required:             []string{},
		AdditionalProperties: false, // openai requires this to be false
	}
	switch bellmanSchema.Type {
	case schema.Object:
		def.Properties = newNilMap()
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
		def.Properties = newNilMap()
		for key, prop := range bellmanSchema.Properties {
			(*def.Properties)[key] = *fromBellmanSchema(prop)
			if (*def.Properties)[key].IsObjectRequired() {
				def.Required = append(def.Required, key)
			}
		}
	}
	if bellmanSchema.Items != nil {
		def.Items = fromBellmanSchema(bellmanSchema.Items)
	}

	if bellmanSchema.Nullable {
		def.Type = []any{def.Type, Null}
	}

	if len(bellmanSchema.Enum) > 0 {
		def.Enum = make([]any, len(bellmanSchema.Enum))
		for i, e := range bellmanSchema.Enum {
			def.Enum[i] = e
		}
	}

	if bellmanSchema.Defs != nil && len(bellmanSchema.Defs) > 0 {
		def.Defs = make(map[string]*JSONSchema)
		for key, prop := range bellmanSchema.Defs {
			def.Defs[key] = fromBellmanSchema(prop)
		}
	}
	if bellmanSchema.Format != nil {
		def.Format = *bellmanSchema.Format
	}
	if bellmanSchema.Pattern != nil {
		def.Pattern = *bellmanSchema.Pattern
	}
	if bellmanSchema.Maximum != nil {
		def.Maximum = *bellmanSchema.Maximum
	}
	if bellmanSchema.Minimum != nil {
		def.Minimum = *bellmanSchema.Minimum
	}
	if bellmanSchema.MaxLength != nil {
		def.MaxLength = *bellmanSchema.MaxLength
	}
	if bellmanSchema.MinLength != nil {
		def.MinLength = *bellmanSchema.MinLength
	}
	if bellmanSchema.MaxItems != nil {
		def.MaxItems = *bellmanSchema.MaxItems
	}
	if bellmanSchema.MinItems != nil {
		def.MinItems = *bellmanSchema.MinItems
	}

	return def
}

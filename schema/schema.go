package schema

//https://platform.openai.com/docs/guides/structured-outputs#supported-schemas
//String
//Number
//Boolean
//Integer
//Object
//Array
//Enum
//anyOf

// https://ai.google.dev/gemini-api/docs/structured-output?lang=python
// int
// float
// bool
// str (or enum)
// list[AllowedType]
// dict[str, AllowedType]
//
// anyOf
// enum
// format
// items
// maximum
// minimum
// maxItems
// minItems
// nullable
// properties
// propertyOrdering*
// required

type JSONType string

const (
	Object  JSONType = "object"
	Array   JSONType = "array"
	String  JSONType = "string"
	Number  JSONType = "number"
	Integer JSONType = "integer"
	Boolean JSONType = "boolean"
)

type JSON struct {
	Ref string `json:"$ref,omitempty"` // #/$defs/... etc, overrides everything else

	// JSON Metadata
	Description string `json:"description,omitempty"`

	// Type System
	Type     JSONType `json:"type,omitempty"`
	Nullable bool     `json:"nullable,omitempty"`

	// Combinators
	Properties           map[string]*JSON `json:"properties,omitempty"`           // for Object
	AdditionalProperties *JSON            `json:"additionalProperties,omitempty"` // for Map[string]someting...
	Items                *JSON            `json:"items,omitempty"`                // for Array

	// Validation
	Enum     []interface{} `json:"enum,omitempty"`
	Required []string      `json:"required,omitempty"`

	/// Number Validation
	Maximum          *float64 `json:"maximum,omitempty"`
	Minimum          *float64 `json:"minimum,omitempty"`
	ExclusiveMaximum *float64 `json:"exclusiveMaximum,omitempty"`
	ExclusiveMinimum *float64 `json:"exclusiveMinimum,omitempty"`

	/// String Validation
	MaxLength *int `json:"maxLength,omitempty"`
	MinLength *int `json:"minLength,omitempty"`

	// Array Validation
	MaxItems *int `json:"maxItems,omitempty"`
	MinItems *int `json:"minItems,omitempty"`
}

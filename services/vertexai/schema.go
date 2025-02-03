package vertexai

import (
	"github.com/modfin/bellman/schema"
)

// https://ai.google.dev/gemini-api/docs/structured-output?lang=rest

type Type string

const (
	Object  Type = "OBJECT"
	Number  Type = "NUMBER"
	Integer Type = "INTEGER"
	String  Type = "STRING"
	Array   Type = "ARRAY"
	Boolean Type = "BOOLEAN"
)

// JSONSchema is used to define the format of input/output data. Represents a select
// subset of an [OpenAPI 3.0 schema
// object](https://spec.openapis.org/oas/v3.0.3#schema). More fields may be
// added in the future as needed.
type JSONSchema struct {
	// Optional. The type of the data.
	Type Type `json:"type,omitempty"`
	// Optional. The format of the data.
	// Supported formats:
	//
	//	for NUMBER type: "float", "double"
	//	for INTEGER type: "int32", "int64"
	//	for STRING type: "email", "byte", etc
	Format string `json:"format,omitempty"`
	// Optional. The title of the Schema.
	Title string `json:"title,omitempty"`
	// Optional. The description of the data.
	Description string `json:"description,omitempty"`
	// Optional. Indicates if the value may be null.
	Nullable bool `json:"nullable,omitempty"`
	// Optional. SCHEMA FIELDS FOR TYPE ARRAY
	// Schema of the elements of Type.ARRAY.
	Items *JSONSchema `json:"items,omitempty"`
	// Optional. Minimum number of the elements for Type.ARRAY.
	MinItems int `json:"min_items,omitempty"`
	// Optional. Maximum number of the elements for Type.ARRAY.
	MaxItems int `json:"max_items,omitempty"`
	// Optional. Possible values of the element of Type.STRING with enum format.
	// For example we can define an Enum Direction as :
	// {type:STRING, format:enum, enum:["EAST", NORTH", "SOUTH", "WEST"]}
	Enum []string `json:"enum, omitempty"`
	// Optional. SCHEMA FIELDS FOR TYPE OBJECT
	// Properties of Type.OBJECT.
	Properties map[string]*JSONSchema `json:"properties,omitempty"`
	// Optional. Required properties of Type.OBJECT.
	Required []string `json:"required,omitempty"`
	// Optional. Minimum number of the properties for Type.OBJECT.
	MinProperties int `json:"min_properties,omitempty"`
	// Optional. Maximum number of the properties for Type.OBJECT.
	MaxProperties int `json:"max_properties,omitempty"`
	// Optional. SCHEMA FIELDS FOR TYPE INTEGER and NUMBER
	// Minimum value of the Type.INTEGER and Type.NUMBER
	Minimum float64 `json:"minimum,omitempty"`
	// Optional. Maximum value of the Type.INTEGER and Type.NUMBER
	Maximum float64 `json:"maximum,omitempty"`
	// Optional. SCHEMA FIELDS FOR TYPE STRING
	// Minimum length of the Type.STRING
	MinLength int `json:"min_length,omitempty"`
	// Optional. Maximum length of the Type.STRING
	MaxLength int `json:"max_length,omitempty"`
	// Optional. Pattern of the Type.STRING to restrict a string to a regular
	// expression.
	Pattern string `json:"pattern,omitempty"`
}

func fromBellmanSchema(bellmanSchema *schema.JSON) *JSONSchema {
	def := &JSONSchema{
		Description: bellmanSchema.Description,
		Required:    bellmanSchema.Required,
		Nullable:    bellmanSchema.Nullable,
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
		def.Properties = make(map[string]*JSONSchema)
		for key, prop := range bellmanSchema.Properties {
			def.Properties[key] = fromBellmanSchema(prop)
		}
	}
	if bellmanSchema.Items != nil {
		def.Items = fromBellmanSchema(bellmanSchema.Items)
	}

	if len(bellmanSchema.Enum) > 0 {
		def.Enum = make([]string, 0)
		for _, e := range bellmanSchema.Enum {
			switch e.(type) {
			case string:
				def.Enum = append(def.Enum, e.(string))
			}
		}
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

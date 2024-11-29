package schema

import (
	"reflect"
	"strconv"
	"strings"
)

func Of[T any]() *JSON {
	var v T
	return New(v)
}

// New converts a struct to a JSON JSON using reflection and struct tags
func New(v interface{}) *JSON {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return typeToSchema(t)
}

func typeToSchema(t reflect.Type) *JSON {
	schema := &JSON{}

	switch t.Kind() {
	case reflect.Map:
		schema.Type = "object"
		schema.Properties = make(map[string]*JSON)
		schema.AdditionalProperties = typeToSchema(t.Elem()) // The value type of the map, key is at t.Key()

	case reflect.Struct:
		schema.Type = "object"
		schema.Properties = make(map[string]*JSON)
		schema.Required = []string{}

		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)

			// Skip unexported fields
			if !field.IsExported() {
				continue
			}

			// Get the JSON field name from the json tag
			jsonTag := field.Tag.Get("json")
			name := strings.Split(jsonTag, ",")[0]
			if name == "-" {
				continue
			}
			if name == "" {
				name = field.Name
			}

			// Check if this field is required
			if !strings.Contains(jsonTag, "omitempty") {
				schema.Required = append(schema.Required, name)
			}

			fieldSchema := fieldToSchema(field)
			if fieldSchema != nil {
				schema.Properties[name] = fieldSchema
			}
		}

		if len(schema.Required) == 0 {
			schema.Required = nil
		}

	case reflect.Slice, reflect.Array:
		schema.Type = "array"
		schema.Items = typeToSchema(t.Elem())

	case reflect.String:
		schema.Type = "string"

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		schema.Type = "integer"

	case reflect.Float32, reflect.Float64:
		schema.Type = "number"

	case reflect.Bool:
		schema.Type = "boolean"
	}

	return schema
}

func fieldToSchema(field reflect.StructField) *JSON {
	schema := typeToSchema(field.Type)

	// Override with field-specific tags
	if desc := field.Tag.Get("json-description"); desc != "" {
		schema.Description = desc
	}
	if typeName := field.Tag.Get("json-type"); typeName != "" {
		schema.Type = typeName
	}

	// Handle number validation for fields
	if schema.Type == "number" || schema.Type == "integer" {
		if incmax := getFloat64Ptr(field.Tag.Get("json-maximum")); incmax != nil {
			schema.Maximum = incmax
		}
		if incmin := getFloat64Ptr(field.Tag.Get("json-minimum")); incmin != nil {
			schema.Minimum = incmin
		}
		if excMax := getFloat64Ptr(field.Tag.Get("json-exclusive-maximum")); excMax != nil {
			schema.ExclusiveMaximum = excMax
		}
		if excMin := getFloat64Ptr(field.Tag.Get("json-exclusive-minimum")); excMin != nil {
			schema.ExclusiveMinimum = excMin
		}

	}

	if schema.Type == "array" {
		if maxItems := getIntFromField(field, "json-max-items"); maxItems != nil {
			schema.MaxItems = maxItems
		}
		if minItems := getIntFromField(field, "json-min-items"); minItems != nil {
			schema.MinItems = minItems
		}
	}

	// Handle string validation for fields
	if schema.Type == "string" {
		if maxLen := getIntFromField(field, "json-max-length"); maxLen != nil {
			schema.MaxLength = maxLen
		}
		if minLen := getIntFromField(field, "json-min-length"); minLen != nil {
			schema.MinLength = minLen
		}
	}

	// Handle enum for fields
	if enum := field.Tag.Get("json-enum"); enum != "" {
		schema.Enum = parseEnum(enum, field.Type.Kind())
	}

	return schema
}

// Helper functions
func getIntFromField(f reflect.StructField, key string) *int {
	if v := f.Tag.Get(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return &i
		}
	}
	return nil
}

func getFloat64Ptr(v string) *float64 {
	if v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return &f
		}
	}
	return nil
}

func parseEnum(enumStr string, kind reflect.Kind) []interface{} {
	values := strings.Split(enumStr, ",")
	enum := make([]interface{}, len(values))

	for i, v := range values {
		v = strings.TrimSpace(v)
		switch kind {
		case reflect.String:
			enum[i] = v
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if n, err := strconv.ParseInt(v, 10, 64); err == nil {
				enum[i] = n
			}
		case reflect.Float32, reflect.Float64:
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				enum[i] = f
			}
		case reflect.Bool:
			if b, err := strconv.ParseBool(v); err == nil {
				enum[i] = b
			}
		}
	}
	return enum
}

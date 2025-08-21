package schema

import (
	"reflect"
	"strconv"
	"strings"
)

// From converts a struct to a JSON using reflection and struct tags
func From(v interface{}) *JSON {
	t := reflect.TypeOf(v)
	var nullable bool
	if t.Kind() == reflect.Ptr {
		nullable = true
		t = t.Elem()
	}
	schema := typeToSchema(t)
	schema.Nullable = nullable
	return schema
}

func typeToSchema(t reflect.Type) *JSON {
	schema := &JSON{}

	if t.Kind() == reflect.Ptr {
		t = t.Elem()
		schema.Nullable = true
	}
	switch t.Kind() {
	case reflect.Map:
		schema.Type = Object
		schema.Properties = make(map[string]*JSON)
		schema.AdditionalProperties = typeToSchema(t.Elem()) // The value type of the map, key is at t.Key()

	case reflect.Struct:
		schema.Type = Object
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
		schema.Type = Array
		schema.Items = typeToSchema(t.Elem())

	case reflect.String:
		schema.Type = String

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		schema.Type = Integer

	case reflect.Float32, reflect.Float64:
		schema.Type = Number

	case reflect.Bool:
		schema.Type = Boolean
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
		schema.Type = JSONType(typeName)
	}

	if schema.Type == "array" {
		if maxItems := getIntFromField(field, "json-max-items"); maxItems != nil {
			schema.MaxItems = maxItems
		}
		if minItems := getIntFromField(field, "json-min-items"); minItems != nil {
			schema.MinItems = minItems
		}
		applyValidationTagsToSchema(schema.Items, field)
	} else {
		// For non-array types, apply validations directly to the main schema
		applyValidationTagsToSchema(schema, field)
	}
	return schema
}

func applyValidationTagsToSchema(schema *JSON, field reflect.StructField) {
	if schema == nil {
		return
	}

	// Handle number validation
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

	// Handle string validation
	if schema.Type == "string" {
		if maxLen := getIntFromField(field, "json-max-length"); maxLen != nil {
			schema.MaxLength = maxLen
		}
		if minLen := getIntFromField(field, "json-min-length"); minLen != nil {
			schema.MinLength = minLen
		}
		if format := field.Tag.Get("json-format"); format != "" {
			schema.Format = &format
		}
		if pattern := field.Tag.Get("json-pattern"); pattern != "" {
			schema.Pattern = &pattern
		}
	}

	// Handle enum validation
	if enumStr := field.Tag.Get("json-enum"); enumStr != "" {
		switch schema.Type {
		case "string", "number", "integer", "boolean":
			schema.Enum = parseEnum(enumStr, field)
		}
	}
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

func parseEnum(enumStr string, field reflect.StructField) []interface{} {
	values := strings.Split(enumStr, ",")
	enum := make([]interface{}, len(values))

	t := field.Type
	kind := t.Kind()
	if kind == reflect.Ptr {
		t = t.Elem()
		kind = t.Kind()
	}

	if kind == reflect.Slice {
		kind = t.Elem().Kind()
	}

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

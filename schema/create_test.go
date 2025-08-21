package schema_test

import (
	"github.com/modfin/bellman/schema"
	"reflect"
	"testing"
)

func TestOf(t *testing.T) {
	type Address struct {
		Street  string `json:"street" json-description:"The street address"`
		Number  int    `json:"number" json-minimum:"1"`
		ZipCode string `json:"zip_code,omitempty"`
	}

	type Person struct {
		Name      string             `json:"name" json-min-length:"1" json-max-length:"100"`
		Age       int                `json:"age" json-minimum:"0" json-maximum:"150"`
		Email     *string            `json:"email" json-type:"string"`
		Address   Address            `json:"address"`
		Addresses []Address          `json:"addresses" json-min-items:"2"`
		Tags      *[]string          `json:"tags" json-min-items:"1" json-min-length:"1" json-max-length:"50" json-pattern:"^[a-zA-Z0-9_]+$" json-enum:"tag1,tag2,tag3"`
		Status    string             `json:"status" json-enum:"active,inactive,pending"`
		Ints      int                `json:"ints" json-enum:"1,2,3"`
		Labels    []string           `json:"labels" json-enum:"Ecstatic,Happy,Sad"`
		AddrMap   map[string]Address `json:"map"`
		Strmap    map[string]float64 `json:"map2"`
	}
	expected := &schema.JSON{
		Type: schema.Object,
		Properties: map[string]*schema.JSON{
			"name":  {Type: schema.String, MinLength: ptr(1), MaxLength: ptr(100)},
			"age":   {Type: schema.Integer, Minimum: ptr(0.), Maximum: ptr(150.)},
			"email": {Type: schema.String, Nullable: true},
			"address": {
				Type: schema.Object,
				Properties: map[string]*schema.JSON{
					"street":   {Type: schema.String, Description: "The street address"},
					"number":   {Type: schema.Integer, Minimum: ptr(1.)},
					"zip_code": {Type: schema.String},
				},
				Required: []string{"street", "number"},
			},
			"addresses": {
				Type: schema.Array,
				Items: &schema.JSON{
					Type: schema.Object,
					Properties: map[string]*schema.JSON{
						"street":   {Type: schema.String, Description: "The street address"},
						"number":   {Type: schema.Integer, Minimum: ptr(1.)},
						"zip_code": {Type: schema.String},
					},
					Required: []string{"street", "number"},
				},
				MinItems: ptr(2),
			},
			"tags":   {Type: schema.Array, Items: &schema.JSON{Type: schema.String, MinLength: ptr(1), MaxLength: ptr(50), Pattern: ptr("^[a-zA-Z0-9_]+$"), Enum: []any{"tag1", "tag2", "tag3"}}, MinItems: ptr(1), Nullable: true},
			"status": {Type: schema.String, Enum: []interface{}{"active", "inactive", "pending"}},
			"ints":   {Type: schema.Integer, Enum: []interface{}{int64(1), int64(2), int64(3)}},
			"labels": {
				Type: schema.Array,
				Items: &schema.JSON{
					Type: schema.String,
					Enum: []interface{}{"Ecstatic", "Happy", "Sad"},
				},
			},
			"map": {
				Type:       schema.Object,
				Properties: map[string]*schema.JSON{},
				AdditionalProperties: &schema.JSON{
					Type: schema.Object,
					Properties: map[string]*schema.JSON{
						"street":   {Type: schema.String, Description: "The street address"},
						"number":   {Type: schema.Integer, Minimum: ptr(1.)},
						"zip_code": {Type: schema.String},
					},
					Required: []string{"street", "number"},
				},
			},
			"map2": {Type: schema.Object, AdditionalProperties: &schema.JSON{Type: schema.Number}, Properties: map[string]*schema.JSON{}},
		},
		Required: []string{"name", "age", "email", "address", "addresses", "tags", "status", "ints", "labels", "map", "map2"},
	}

	result := schema.From(Person{})
	if !reflect.DeepEqual(result.Properties, expected.Properties) {
		t.Errorf("Expected %+v, got %+v", expected, result)
	}

}

func TestFrom_PrimitiveTypes(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected *schema.JSON
	}{
		{
			name:  "string",
			input: "test",
			expected: &schema.JSON{
				Type: schema.String,
			},
		},
		{
			name:  "pointer to string",
			input: new(string),
			expected: &schema.JSON{
				Type:     schema.String,
				Nullable: true,
			},
		},
		{
			name:  "integer",
			input: 42,
			expected: &schema.JSON{
				Type: schema.Integer,
			},
		},
		{
			name:  "pointer to integer",
			input: new(int),
			expected: &schema.JSON{
				Type:     schema.Integer,
				Nullable: true,
			},
		},
		{
			name:  "float",
			input: 3.14,
			expected: &schema.JSON{
				Type: schema.Number,
			},
		},
		{
			name:  "pointer to float",
			input: new(float64),
			expected: &schema.JSON{
				Type:     schema.Number,
				Nullable: true,
			},
		},
		{
			name:  "boolean",
			input: true,
			expected: &schema.JSON{
				Type: schema.Boolean,
			},
		},
		{
			name:  "pointer to boolean",
			input: new(bool),
			expected: &schema.JSON{
				Type:     schema.Boolean,
				Nullable: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := schema.From(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("Expected %+v, got %+v", tt.expected, result)
			}
		})
	}
}

func TestFrom_Struct(t *testing.T) {
	type TestStruct struct {
		Name string `json:"name"`
		Age  int    `json:"age,omitempty"`
	}

	expected := &schema.JSON{
		Type: schema.Object,
		Properties: map[string]*schema.JSON{
			"name": {Type: schema.String},
			"age":  {Type: schema.Integer},
		},
		Required: []string{"name"},
	}

	result := schema.From(TestStruct{})
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %+v, got %+v", expected, result)
	}
}

func TestFrom_NestedStruct(t *testing.T) {
	type Inner struct {
		X int `json:"x"`
	}
	type Outer struct {
		Inner Inner `json:"inner"`
	}

	expected := &schema.JSON{
		Type: schema.Object,
		Properties: map[string]*schema.JSON{
			"inner": {
				Type: schema.Object,
				Properties: map[string]*schema.JSON{
					"x": {Type: schema.Integer},
				},
				Required: []string{"x"},
			},
		},
		Required: []string{"inner"},
	}

	result := schema.From(Outer{})
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %+v, got %+v", expected, result)
	}
}

func TestFrom_Map(t *testing.T) {
	input := map[string]int{}
	expected := &schema.JSON{
		Type:                 schema.Object,
		AdditionalProperties: &schema.JSON{Type: schema.Integer},
		Properties:           map[string]*schema.JSON{},
	}

	result := schema.From(input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected \n%+v\n, got \n%+v\n", expected, result)
	}
}

func TestFrom_SliceAndArray(t *testing.T) {
	sliceInput := []int{}
	arrayInput := [3]int{}

	expectedSlice := &schema.JSON{
		Type:  schema.Array,
		Items: &schema.JSON{Type: schema.Integer},
	}

	expectedArray := &schema.JSON{
		Type:  schema.Array,
		Items: &schema.JSON{Type: schema.Integer},
	}

	t.Run("slice", func(t *testing.T) {
		result := schema.From(sliceInput)
		if !reflect.DeepEqual(result, expectedSlice) {
			t.Errorf("Expected %+v, got %+v", expectedSlice, result)
		}
	})

	t.Run("array", func(t *testing.T) {
		result := schema.From(arrayInput)
		if !reflect.DeepEqual(result, expectedArray) {
			t.Errorf("Expected %+v, got %+v", expectedArray, result)
		}
	})
}

func TestFrom_Tags(t *testing.T) {
	type TaggedStruct struct {
		Name        string  `json:"name" json-description:"User's name" json-type:"string"`
		Age         int     `json:"age" json-minimum:"18" json-maximum:"100"`
		Description string  `json:"-"`
		Rate        float64 `json:"rate" json-exclusive-minimum:"0.0" json-exclusive-maximum:"5.0"`
	}

	minAge := 18.0
	maxAge := 100.0
	excMinRate := 0.0
	excMaxRate := 5.0

	expected := &schema.JSON{
		Type: schema.Object,
		Properties: map[string]*schema.JSON{
			"name": {
				Type:        schema.String,
				Description: "User's name",
			},
			"age": {
				Type:    schema.Integer,
				Minimum: &minAge,
				Maximum: &maxAge,
			},
			"rate": {
				Type:             schema.Number,
				ExclusiveMinimum: &excMinRate,
				ExclusiveMaximum: &excMaxRate,
			},
		},
		Required: []string{"name", "age", "rate"},
	}

	result := schema.From(TaggedStruct{})
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %+v, got %+v", expected, result)
	}
}

func TestFrom_Enum(t *testing.T) {
	type EnumStruct struct {
		Color   string  `json:"color" json-enum:"red,green,blue"`
		Status  int     `json:"status" json-enum:"200,404,500"`
		Factor  float64 `json:"factor" json-enum:"1.0,2.0,3.0"`
		Enabled bool    `json:"enabled" json-enum:"true,false"`
	}

	expected := &schema.JSON{
		Type: schema.Object,
		Properties: map[string]*schema.JSON{
			"color": {
				Type: schema.String,
				Enum: []interface{}{"red", "green", "blue"},
			},
			"status": {
				Type: schema.Integer,
				Enum: []interface{}{int64(200), int64(404), int64(500)},
			},
			"factor": {
				Type: schema.Number,
				Enum: []interface{}{1.0, 2.0, 3.0},
			},
			"enabled": {
				Type: schema.Boolean,
				Enum: []interface{}{true, false},
			},
		},
		Required: []string{"color", "status", "factor", "enabled"},
	}

	result := schema.From(EnumStruct{})
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %+v, got %+v", expected, result)
	}
}

func TestFrom_UnexportedFields(t *testing.T) {
	type TestStruct struct {
		name string `json:"name"`
		Age  int    `json:"age"`
	}

	expected := &schema.JSON{
		Type: schema.Object,
		Properties: map[string]*schema.JSON{
			"age": {Type: schema.Integer},
		},
		Required: []string{"age"},
	}

	result := schema.From(TestStruct{})
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %+v, got %+v", expected, result)
	}
}

func TestFrom_IgnoredField(t *testing.T) {
	type TestStruct struct {
		Secret string `json:"-"`
		Public int    `json:"public"`
	}

	expected := &schema.JSON{
		Type: schema.Object,
		Properties: map[string]*schema.JSON{
			"public": {Type: schema.Integer},
		},
		Required: []string{"public"},
	}

	result := schema.From(TestStruct{})
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %+v, got %+v", expected, result)
	}
}

func TestFrom_NullableField(t *testing.T) {
	type TestStruct struct {
		Name *string `json:"name"`
	}

	expected := &schema.JSON{
		Type: schema.Object,
		Properties: map[string]*schema.JSON{
			"name": {
				Type:     schema.String,
				Nullable: true,
			},
		},
		Required: []string{"name"},
	}

	result := schema.From(TestStruct{})
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %+v, got %+v", expected, result)
	}
}

func TestFrom_EmptyStruct(t *testing.T) {
	type EmptyStruct struct{}

	expected := &schema.JSON{
		Type:       schema.Object,
		Properties: map[string]*schema.JSON{},
	}

	result := schema.From(EmptyStruct{})
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %+v, got %+v", expected, result)
	}
}

func TestFrom_SliceOfPointers(t *testing.T) {
	type TestStruct struct {
		List []*int `json:"list"`
	}

	expected := &schema.JSON{
		Type: schema.Object,
		Properties: map[string]*schema.JSON{
			"list": {
				Type:  schema.Array,
				Items: &schema.JSON{Type: schema.Integer, Nullable: true},
			},
		},
		Required: []string{"list"},
	}

	result := schema.From(TestStruct{})
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %+v, got %+v", expected, result)
	}
}

func TestFrom_MapWithPointerValues(t *testing.T) {
	input := map[string]*float64{}
	expected := &schema.JSON{
		Type:                 schema.Object,
		AdditionalProperties: &schema.JSON{Type: schema.Number, Nullable: true},
		Properties:           map[string]*schema.JSON{},
	}

	result := schema.From(input)
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %+v, got %+v", expected, result)
	}
}

func TestFrom_InvalidTags(t *testing.T) {
	type TestStruct struct {
		Age int `json:"age" json-minimum:"invalid" json-max-length:"notanumber"`
	}

	expected := &schema.JSON{
		Type: schema.Object,
		Properties: map[string]*schema.JSON{
			"age": {Type: schema.Integer},
		},
		Required: []string{"age"},
	}

	result := schema.From(TestStruct{})
	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %+v, got %+v", expected, result)
	}
}

func ptr[T any](v T) *T {
	return &v
}

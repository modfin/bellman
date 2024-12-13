package schema_test

import (
	"encoding/json"
	"fmt"
	"github.com/modfin/bellman/schema"
	"testing"
)

// Example usage:
type Address struct {
	Street  string `json:"street" json-description:"The street address"`
	Number  int    `json:"number" json-minimum:"1"`
	ZipCode string `json:"zip_code,omitempty"`
}

type Person struct {
	Name      string             `json:"name" json-min-length:"1" json-max-length:"100"`
	Age       int                `json:"age" json-minimum:"0" json-maximum:"150"`
	Email     *string            `json:"email" json-type:"string" json-format:"email"`
	Address   Address            `json:"address"`
	Addresses []Address          `json:"addresses" json-min-items:"2"`
	Tags      []string           `json:"tags" json-min-items:"1"`
	Status    string             `json:"status" json-enum:"active,inactive,pending"`
	Ints      int                `json:"ints" json-enum:"1,2,3"`
	Labels    []string           `json:"labels" json-enum:"Ecstatic,Happy,Sad"`
	AddrMap   map[string]Address `json:"map"`
	Strmap    map[int]float64    `json:"map2"`
}

func TestOf(t *testing.T) {
	schema := schema.From(Person{})
	d, err := json.MarshalIndent(schema, "", "  ")

	fmt.Println(string(d), err)
}

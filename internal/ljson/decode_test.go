package ljson

import (
	"fmt"
	"testing"
)

func TestUnmarshal(t *testing.T) {
	// Define the expected schema as a struct
	type Nested struct {
		A int `json:"a"`
	}

	type MySchema struct {
		Field1    []map[string]string `json:"field1"`
		NestedObj Nested              `json:"nested"`
		Numbers   int                 `json:"numbers"`
		BoolVal   bool                `json:"bool_val"`
	}

	// JSON with objects stored as strings and type mismatches
	jsonStr := `{
		"field1": "[{\"sub1\": \"xxx\"}, {\"sub2\": \"yyy\"}]",
    "nested": "{\"a\": \"123\"}",
		"numbers": "456",
		"bool_val": "true"
	}`
	arrStr := `[{
		"field1": "[{\"sub1\": \"xxx\"}, {\"sub2\": \"yyy\"}]",
    "nested": "{\"a\": \"123\"}",
		"numbers": "456",
		"bool_val": "true"
  }, {
		"field1": "[{\"sub1\": \"xxx\"}, {\"sub2\": \"yyy\"}]",
    "nested": "{\"a\": \"123\"}",
		"numbers": "456",
		"bool_val": "true"
  }]`
	mapStr := `"{\"sub1\": \"xxx\", \"sub2\": 123}"`

	// Define a struct instance to receive the parsed data
	var result MySchema

	// Unmarshal using our loose parser
	if err := Unmarshal([]byte(jsonStr), &result); err != nil {
		t.Error(err)
		return
	}

	// Print the processed result
	resultJSON, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(resultJSON))

	// Define a struct instance to receive the parsed data
	var arrResult []MySchema

	// Unmarshal using our loose parser
	if err := Unmarshal([]byte(arrStr), &arrResult); err != nil {
		t.Error(err)
		return
	}

	// Print the processed result
	resultJSON, _ = json.MarshalIndent(arrResult, "", "  ")
	fmt.Println(string(resultJSON))

	// Define a struct instance to receive the parsed data
	mapResult := make(map[string]string)

	// Unmarshal using our loose parser
	if err := Unmarshal([]byte(mapStr), &mapResult); err != nil {
		t.Error(err)
		return
	}

	// Print the processed result
	resultJSON, _ = json.MarshalIndent(mapResult, "", "  ")
	fmt.Println(string(resultJSON))
}

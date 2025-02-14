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

	// Example with interface type
	interfaceData := `{
		"some_field": "{\"a\": \"456\"}"
	}`
	var myInterface interface{}
	if err := Unmarshal([]byte(interfaceData), &myInterface); err != nil {
		fmt.Println("Error unmarshalling interface:", err)
	} else {
		fmt.Printf("Unmarshalled interface: %+v\n", myInterface)
	}
}

func TestComplicateUnmarshal(t *testing.T) {
	type Ingredient struct {
		Name          string  `json:"name" jsonschema:"title=name,description=The name of the ingredient/portion for the food item or dish"`
		ServeSize     float64 `json:"serve_size,omitempty" jsonschema:"title=serve_size,description=The serve size of the food for analyzing in milligrams or milliliters."`
		Carlories     float64 `json:"carlories" jsonschema:"title=carlories,description=The calories provided by the ingredient in kcal"`
		Protein       float64 `json:"protein" jsonschema:"title=protein,description=The protein provided by the ingredient in milligrams"`
		Carbohydrates float64 `json:"carbohydrates" jsonschema:"title=carbohydrates,description=The carbohydrates provided by the ingredient in milligrams"`
		Fats          float64 `json:"fats" jsonschema:"title=fats,description=The fats provided by the ingredient in milligrams"`
	}

	type CookingStep struct {
		Name  string   `json:"name" jsonschema:"title=name,description=The description of each main step of the recipe"`
		Steps []string `json:"steps" jsonschema:"title=steps,description=The description of the sub steps of the recipe"`
	}

	type Recipe struct {
		Name        string        `json:"name" jsonschema:"title=name,description=The full name or description of the recipe"`
		Ingredients []Ingredient  `json:"ingredients" jsonschema:"title=ingredients,description=A list of different ingredients or components in the dish and food item"`
		Steps       []CookingStep `json:"steps" jsonschema:"title=steps,description=A list of different steps and key points to make the recipe"`
	}

	type Output struct {
		Recipe *Recipe `json:"recipe" jsonschema:"title=recipe,description=The recipe info of dish or food item being asked."`
	}

	txt := `{
  "recipe": {
    "name": "鱼香肉丝 (Fish-flavored Shredded Pork)",
    "ingredients": [
      "{\"name\":\"猪里脊肉\",\"serve_size\":150000,\"unit\":\"mg\",\"carlories\":246.0,\"protein\":26000.0,\"carbohydrates\":0.0,\"fats\":15000.0}",
      "{\"name\":\"胡萝卜\",\"serve_size\":50000,\"unit\":\"mg\",\"carlories\":21.0,\"protein\":1000.0,\"carbohydrates\":4000.0,\"fats\":100.0}",
      "{\"name\":\"青椒\",\"serve_size\":50000,\"unit\":\"mg\",\"carlories\":20.0,\"protein\":1000.0,\"carbohydrates\":4000.0,\"fats\":0.0}",
      "{\"name\":\"木耳\",\"serve_size\":10000,\"unit\":\"mg\",\"carlories\":29.0,\"protein\":2600.0,\"carbohydrates\":5000.0,\"fats\":0.0}",
      "{\"name\":\"姜\",\"serve_size\":5000,\"unit\":\"mg\",\"carlories\":14.0,\"protein\":200.0,\"carbohydrates\":3000.0,\"fats\":0.0}",
      "{\"name\":\"蒜\",\"serve_size\":5000,\"unit\":\"mg\",\"carlories\":18.0,\"protein\":100.0,\"carbohydrates\":4000.0,\"fats\":0.0}",
      "{\"name\":\"大葱\",\"serve_size\":10000,\"unit\":\"mg\",\"carlories\":32.0,\"protein\":1000.0,\"carbohydrates\":7000.0,\"fats\":0.0}",
      "{\"name\":\"酱油\",\"serve_size\":10000,\"unit\":\"ml\",\"carlories\":60.0,\"protein\":2000.0,\"carbohydrates\":5000.0,\"fats\":0.0}",
      "{\"name\":\"醋\",\"serve_size\":10000,\"unit\":\"ml\",\"carlories\":20.0,\"protein\":0.0,\"carbohydrates\":5000.0,\"fats\":0.0}",
      "{\"name\":\"糖\",\"serve_size\":10000,\"unit\":\"mg\",\"carlories\":38.0,\"protein\":0.0,\"carbohydrates\":10000.0,\"fats\":0.0}",
      "{\"name\":\"料酒\",\"serve_size\":10000,\"unit\":\"ml\",\"carlories\":20.0,\"protein\":0.0,\"carbohydrates\":2000.0,\"fats\":0.0}",
      "{\"name\":\"淀粉\",\"serve_size\":10000,\"unit\":\"mg\",\"carlories\":35.0,\"protein\":1000.0,\"carbohydrates\":8000.0,\"fats\":0.0}",
      "{\"name\":\"盐\",\"serve_size\":2000,\"unit\":\"mg\",\"carlories\":0.0,\"protein\":0.0,\"carbohydrates\":0.0,\"fats\":0.0}",
      "{\"name\":\"食用油\",\"serve_size\":30000,\"unit\":\"mg\",\"carlories\":270.0,\"protein\":0.0,\"carbohydrates\":0.0,\"fats\":30000.0}"
    ],
    "steps": [
      "{\"name\":\"准备食材\",\"steps\":[\"将猪里脊肉切成细丝\",\"胡萝卜、青椒切丝\",\"木耳泡发后切丝\",\"姜蒜切末，大葱切段\"]}",
      "{\"name\":\"腌制肉丝\",\"steps\":[\"在肉丝中加入少许盐\",\"加入料酒和淀粉搅拌均匀\",\"静置腌制15分钟\"]}",
      "{\"name\":\"调汁\",\"steps\":[\"在一个碗中混合酱油、醋、糖、料酒、少量水和淀粉\",\"搅拌均匀备用\"]}",
      "{\"name\":\"炒制\",\"steps\":[\"热锅凉油，待油温升至五成热\",\"下入腌制好的肉丝快速滑散\",\"待肉丝变色后盛出备用\",\"锅中留底油，放入姜蒜末和大葱段爆香\",\"依次加入胡萝卜丝、青椒丝、木耳丝翻炒\",\"将肉丝倒回锅中\",\"倒入调好的酱汁，翻炒均匀\",\"最后加入适量盐调味即可\"]}"
    ]
  }
}`
	var output Output
	// Unmarshal using our loose parser
	if err := Unmarshal([]byte(txt), &output); err != nil {
		t.Error(err)
		return
	}

	// Print the processed result
	resultJSON, _ := json.MarshalIndent(output, "", "  ")
	fmt.Println(string(resultJSON))
}

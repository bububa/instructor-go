package yaml

import (
	"fmt"
	"testing"
)

func TestYaml(t *testing.T) {
	type Details struct {
		Location string `yaml:"location" jsonschema:"description=location" fake:"Beijing"`
		Gender   string `yaml:"gender" jsonschema:"description=gender" fake:"male"`
	}

	type Person struct {
		Name       string    `yaml:"name" comment:"这里是姓名字段" jsonschema:"description=用户的名字" fake:"Syd Xu"`
		Age        *int      `yaml:"age" jsonschema:"description=用户的年龄" fake:"24"`
		Details    *Details  `yaml:"details" jsonschema:"description=附加的详细信息"`
		DetailList []Details `yaml:"details" jsonschema:"description=附加的详细信息数组" fakesize="2"`
	}
	var p Person
	enc := NewEncoder(p).WithCommentStyle(LineComment)
	fmt.Println(string(enc.Context()))
	// Output:
	// Please respond with YAML in the following YAML schema:
	//
	// ```yaml
	// name: Syd Xu # 这里是姓名字段
	// age: 24 # 用户的年龄
	// details: # 附加的详细信息
	//     location: Beijing # location
	//     gender: male # gender
	// details: # 附加的详细信息数组
	//     - location: Beijing # location
	//       gender: male # gender
	//     - location: Beijing # location
	//       gender: male # gender
	//     - location: Beijing # location
	//       gender: male # gender
	//     - location: Beijing # location
	//       gender: male # gender
	// ```
	// Make sure to return an instance of the YAML, not the schema itself
	//
}

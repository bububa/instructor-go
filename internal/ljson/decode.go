package ljson

import (
	"fmt"
	"reflect"

	jsoniter "github.com/json-iterator/go"
	"github.com/spf13/cast"
)

var json = jsoniter.ConfigCompatibleWithStandardLibrary

// Unmarshal function that processes JSON loosely based on a schema
func Unmarshal(data []byte, schema interface{}) error {
	if err := json.Unmarshal(data, schema); err == nil {
		return nil
	}
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("error unmarshalling JSON: %w", err)
	}

	schemaValue := reflect.ValueOf(schema).Elem()
	schemaType := schemaValue.Type()

	switch schemaType.Kind() {
	case reflect.Slice:
		if jsonString, ok := raw.(string); ok && isJSONString(jsonString) {
			return Unmarshal([]byte(jsonString), schema)
		}
		// Handle case when the schema is a slice
		sliceValue := reflect.MakeSlice(schemaType, 0, 0)
		rawArray, ok := raw.([]interface{})
		if !ok {
			return fmt.Errorf("expected an array in the input data")
		}

		for _, item := range rawArray {
			// Create a new element for the slice and unmarshal into it
			newElem := reflect.New(schemaType.Elem()).Interface()
			if dataBytes, err := json.Marshal(item); err != nil {
				return err
			} else if err := Unmarshal(dataBytes, newElem); err != nil {
				return err
			}

			sliceValue = reflect.Append(sliceValue, reflect.ValueOf(newElem).Elem())
		}
		schemaValue.Set(sliceValue)
		return nil
	case reflect.Map:
		if jsonString, ok := raw.(string); ok && isJSONString(jsonString) {
			return Unmarshal([]byte(jsonString), schema)
		}
		mapValue := reflect.MakeMap(schemaType)
		rawMap, ok := raw.(map[string]interface{})
		if !ok {
			return fmt.Errorf("expected an map in the input data")
		}

		for key, item := range rawMap {
			// Create a new element for the slice and unmarshal into it
			mapKey := reflect.ValueOf(key)
			newElem := reflect.New(schemaType.Elem()).Interface()
			if dataBytes, err := json.Marshal(item); err != nil {
				return err
			} else if err := Unmarshal(dataBytes, newElem); err != nil {
				return err
			}

			mapValue.SetMapIndex(mapKey, reflect.ValueOf(newElem).Elem())
		}
		schemaValue.Set(mapValue)
		return nil
	case reflect.Struct:
		if jsonString, ok := raw.(string); ok && isJSONString(jsonString) {
			return Unmarshal([]byte(jsonString), schema)
		}
		// Handle case when the schema is a struct
		for key, value := range raw.(map[string]interface{}) {
			field := findFieldByJSONTag(schemaValue, key)
			if field.IsValid() && field.CanSet() {
				// Process the value based on the schema type
				processedValue := processValue(value, field.Type())
				field.Set(reflect.ValueOf(processedValue))
			}
		}
		return nil
	case reflect.String:
		schemaValue.SetString(cast.ToString(raw))
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		schemaValue.SetInt(cast.ToInt64(raw))
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		schemaValue.SetUint(cast.ToUint64(raw))
	case reflect.Float32, reflect.Float64:
		schemaValue.SetFloat(cast.ToFloat64(raw))
	case reflect.Bool:
		schemaValue.SetBool(cast.ToBool(raw))
	}

	return fmt.Errorf("unsupported schema type: %s", schemaType.Kind())
}

// UnmarshalDataIntoValue: unmarshals raw data into the provided value based on the schema
func UnmarshalDataIntoValue(data interface{}, value interface{}) error {
	valueType := reflect.TypeOf(value).Elem()

	// If it's a struct, unmarshal into it
	if valueType.Kind() == reflect.Struct {
		dataBytes, err := json.Marshal(data)
		if err != nil {
			return err
		}
		return Unmarshal(dataBytes, value)
	}

	// If it's a primitive type, do the type conversion
	valueElem := reflect.ValueOf(value).Elem()
	processedValue := processValue(data, valueElem.Type())
	valueElem.Set(reflect.ValueOf(processedValue))

	return nil
}

// Find struct field by its JSON tag (case-insensitive search)
func findFieldByJSONTag(v reflect.Value, jsonTag string) reflect.Value {
	// Check if the struct field matches the JSON tag
	for i := 0; i < v.NumField(); i++ {
		field := v.Type().Field(i)
		tag := field.Tag.Get("json")

		// If the JSON tag matches the provided key, return the field
		if tag == jsonTag || tag == jsonTag+",omitempty" {
			return v.Field(i)
		}
	}
	return reflect.Value{}
}

// Converts values to match expected schema types
func processValue(value interface{}, expectedType reflect.Type) interface{} {
	// Handle stringified JSON arrays or objects
	if expectedType.Kind() == reflect.Slice || expectedType.Kind() == reflect.Map || expectedType.Kind() == reflect.Struct {
		if jsonString, ok := value.(string); ok && isJSONString(jsonString) {
			newValue := reflect.New(expectedType).Interface()
			if err := Unmarshal([]byte(jsonString), newValue); err != nil {
				return err
			}
			// Recursively process nested structs/maps/slices
			processSchema(reflect.ValueOf(newValue))
			return reflect.ValueOf(newValue).Elem().Interface()
		}
	}

	// Use `cast` for primitive type conversion
	switch expectedType.Kind() {
	case reflect.String:
		return cast.ToString(value)
	case reflect.Int:
		return cast.ToInt(value)
	case reflect.Int8:
		return cast.ToInt8(value)
	case reflect.Int16:
		return cast.ToInt16(value)
	case reflect.Int32:
		return cast.ToInt32(value)
	case reflect.Int64:
		return cast.ToInt64(value)
	case reflect.Uint:
		return cast.ToUint(value)
	case reflect.Uint8:
		return cast.ToUint8(value)
	case reflect.Uint16:
		return cast.ToUint16(value)
	case reflect.Uint32:
		return cast.ToUint32(value)
	case reflect.Uint64:
		return cast.ToUint64(value)
	case reflect.Float32:
		return cast.ToFloat32(value)
	case reflect.Float64:
		return cast.ToFloat64(value)
	case reflect.Bool:
		return cast.ToBool(value)
	}

	// Handle pointer types
	if expectedType.Kind() == reflect.Ptr && !reflect.ValueOf(value).IsNil() {
		return reflect.New(expectedType.Elem()).Interface()
	}

	return value // Return as-is if no conversion needed
}

// Recursively processes the schema and fixes type mismatches
func processSchema(v reflect.Value) {
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return
	}
	v = v.Elem()

	switch v.Kind() {
	case reflect.Map:
		for _, key := range v.MapKeys() {
			val := v.MapIndex(key)
			if val.CanInterface() {
				processedValue := processValue(val.Interface(), v.Type().Elem())
				v.SetMapIndex(key, reflect.ValueOf(processedValue))
			}
		}

	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			elem := v.Index(i)
			if elem.CanInterface() {
				processedValue := processValue(elem.Interface(), v.Type().Elem())
				v.Index(i).Set(reflect.ValueOf(processedValue))
			}
		}

	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			if field.CanSet() {
				processedValue := processValue(field.Interface(), field.Type())
				field.Set(reflect.ValueOf(processedValue))
			}
		}
	}
}

// Detects if a value is a JSON stringified object
func isJSONString(str string) bool {
	if len(str) < 2 || (str[0] != '{' && str[0] != '[') {
		return false
	}
	var temp interface{}
	return json.Unmarshal([]byte(str), &temp) == nil
}

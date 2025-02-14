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
	schemaValue := reflect.ValueOf(schema)
	schemaType := schemaValue.Type()

	// Check if the schema is a pointer and get the underlying value
	if schemaValue.Kind() != reflect.Ptr {
		return fmt.Errorf("schema must be a pointer")
	}
	if err := json.Unmarshal(data, schema); err == nil {
		return nil
	}

	// Dereference the pointer to get the underlying value if it's a pointer
	if schemaValue.IsNil() {
		schemaValue.Set(reflect.New(schemaType.Elem()))
	}

	if err := json.Unmarshal(data, schema); err == nil {
		return nil
	}

	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("error unmarshalling JSON: %w", err)
	}

	// Dereference the pointer to get the underlying value
	schemaValue = schemaValue.Elem()
	schemaType = schemaValue.Type()

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
			// Check if the item is a stringified JSON object
			if jsonString, ok := item.(string); ok && isJSONString(jsonString) {
				// If the item is a stringified JSON, unmarshal it again
				if err := Unmarshal([]byte(jsonString), newElem); err != nil {
					return err
				}
			} else {
				if dataBytes, err := json.Marshal(item); err != nil {
					return err
				} else if err := Unmarshal(dataBytes, newElem); err != nil {
					return err
				}
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
			// Check if the item is a stringified JSON object
			if jsonString, ok := item.(string); ok && isJSONString(jsonString) {
				// If the item is a stringified JSON, unmarshal it again
				if err := Unmarshal([]byte(jsonString), newElem); err != nil {
					return err
				}
			} else {
				if dataBytes, err := json.Marshal(item); err != nil {
					return err
				} else if err := Unmarshal(dataBytes, newElem); err != nil {
					return err
				}
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
				processedValue, err := processValue(value, field.Type())
				if err != nil {
					return err
				}
				field.Set(reflect.ValueOf(processedValue).Convert(field.Type()))
			}
		}
		return nil
	case reflect.Ptr:
		// Allocate the underlying value if it's nil
		if schemaValue.IsNil() {
			schemaValue.Set(reflect.New(schemaType.Elem()))
		}
		// Recursively call Unmarshal for the pointer
		return Unmarshal(data, schemaValue.Interface())
	case reflect.Interface:
		// For interface types, we need to unmarshal directly into the concrete type
		// Unmarshal the raw data into the interface itself
		if raw != nil {
			if err := Unmarshal(data, schemaValue.Interface()); err != nil {
				return err
			}
		} else {
			// Set to nil if raw is nil
			schemaValue.Set(reflect.Zero(schemaType))
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
func processValue(value interface{}, expectedType reflect.Type) (interface{}, error) {
	// Handle stringified JSON arrays or objects
	if expectedType.Kind() == reflect.Slice || expectedType.Kind() == reflect.Map || expectedType.Kind() == reflect.Struct || expectedType.Kind() == reflect.Ptr || expectedType.Kind() == reflect.Interface {
		newValue := reflect.New(expectedType).Interface()
		var (
			bs  []byte
			err error
		)
		if jsonString, ok := value.(string); ok && isJSONString(jsonString) {
			bs = []byte(jsonString)
		} else {
			if bs, err = json.Marshal(value); err != nil {
				return nil, err
			}
		}
		if err := Unmarshal(bs, newValue); err != nil {
			return nil, err
		}
		// Recursively process nested structs/maps/slices
		if err := processSchema(reflect.ValueOf(newValue)); err != nil {
			return nil, err
		}
		return reflect.ValueOf(newValue).Elem().Interface(), nil
	}

	// Use `cast` for primitive type conversion
	switch expectedType.Kind() {
	case reflect.String:
		return cast.ToString(value), nil
	case reflect.Int:
		return cast.ToInt(value), nil
	case reflect.Int8:
		return cast.ToInt8(value), nil
	case reflect.Int16:
		return cast.ToInt16(value), nil
	case reflect.Int32:
		return cast.ToInt32(value), nil
	case reflect.Int64:
		return cast.ToInt64(value), nil
	case reflect.Uint:
		return cast.ToUint(value), nil
	case reflect.Uint8:
		return cast.ToUint8(value), nil
	case reflect.Uint16:
		return cast.ToUint16(value), nil
	case reflect.Uint32:
		return cast.ToUint32(value), nil
	case reflect.Uint64:
		return cast.ToUint64(value), nil
	case reflect.Float32:
		return cast.ToFloat32(value), nil
	case reflect.Float64:
		return cast.ToFloat64(value), nil
	case reflect.Bool:
		return cast.ToBool(value), nil
	case reflect.Ptr:
		// Handle pointer types
		if reflect.ValueOf(value).IsNil() {
			return nil, nil
		}
		return reflect.New(expectedType.Elem()).Interface(), nil
	case reflect.Interface:
		if value == nil {
			return nil, nil
		}
		bs, err := json.Marshal(value)
		if err != nil {
			return nil, err
		}
		// Create a new instance of the concrete type stored in the interface
		concreteValue := reflect.New(expectedType).Interface()
		if err := Unmarshal(bs, concreteValue); err != nil {
			return nil, err
		}
		return concreteValue, nil
	}

	return value, nil // Return as-is if no conversion needed
}

// Recursively processes the schema and fixes type mismatches
func processSchema(v reflect.Value) error {
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return nil
	}
	v = v.Elem()

	switch v.Kind() {
	case reflect.Map:
		for _, key := range v.MapKeys() {
			val := v.MapIndex(key)
			if val.CanInterface() {
				processedValue, err := processValue(val.Interface(), v.Type().Elem())
				if err != nil {
					return err
				}
				v.SetMapIndex(key, reflect.ValueOf(processedValue).Convert(v.Type().Elem()))
			}
		}

	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			elem := v.Index(i)
			if elem.CanInterface() {
				processedValue, err := processValue(elem.Interface(), v.Type().Elem())
				if err != nil {
					return err
				}
				v.Index(i).Set(reflect.ValueOf(processedValue).Convert(v.Type().Elem()))
			}
		}

	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			field := v.Field(i)
			if field.CanSet() {
				processedValue, err := processValue(field.Interface(), field.Type())
				if err != nil {
					return err
				}
				field.Set(reflect.ValueOf(processedValue).Convert(field.Type()))
			}
		}
	}
	return nil
}

// Detects if a value is a JSON stringified object
func isJSONString(str string) bool {
	if len(str) < 2 || (str[0] != '{' && str[0] != '[') {
		return false
	}
	var temp interface{}
	return json.Unmarshal([]byte(str), &temp) == nil
}

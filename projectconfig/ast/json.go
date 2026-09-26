package ast

import (
	"reflect"
	"strings"
)

// JSONValue converts an AST to a JSON-friendly value with a kind discriminator
// on every node.
func JSONValue(node Node) any { return jsonValue(reflect.ValueOf(node)) }

func jsonValue(value reflect.Value) any {
	if !value.IsValid() {
		return nil
	}
	for value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil
		}
		value = value.Elem()
	}
	switch value.Kind() {
	case reflect.Struct:
		result := map[string]any{}
		if value.CanAddr() {
			if _, ok := value.Addr().Interface().(Node); ok {
				result["kind"] = value.Type().Name()
			}
		}
		for i := range value.NumField() {
			info := value.Type().Field(i)
			if !info.IsExported() {
				continue
			}
			field := value.Field(i)
			if info.Anonymous {
				if embedded, ok := jsonValue(field).(map[string]any); ok {
					for key, item := range embedded {
						result[key] = item
					}
				}
				continue
			}
			name, options, _ := strings.Cut(info.Tag.Get("json"), ",")
			if name == "-" {
				continue
			}
			if name == "" {
				name = info.Name
			}
			if strings.Contains(options, "omitempty") && field.IsZero() {
				continue
			}
			result[name] = jsonValue(field)
		}
		return result
	case reflect.Slice, reflect.Array:
		result := make([]any, value.Len())
		for i := range value.Len() {
			result[i] = jsonValue(value.Index(i))
		}
		return result
	case reflect.String:
		return value.String()
	case reflect.Bool:
		return value.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return value.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return value.Uint()
	case reflect.Float32, reflect.Float64:
		return value.Float()
	default:
		if value.CanInterface() {
			return value.Interface()
		}
		return nil
	}
}

package utils

import (
	"fmt"
	"reflect"
)

func StringifyArgument(argument any) string {
	// if argument is nil, return an empty string
	if argument == nil {
		return ""
	}

	// if argument is empty/zero value, return an empty string
	v := reflect.ValueOf(argument)
	if !v.IsValid() || v.IsZero() {
		return ""
	}

	output := ""

	// if the argument is a map, stringify the map
	if argumentMap, ok := argument.(map[string]any); ok {
		for key, value := range argumentMap {
			output += fmt.Sprintf("%s=%v,", key, value)
		}
		// remove trailing comma if present
		if len(output) > 0 {
			output = output[:len(output)-1]
		}
		return output
	}

	// if argument is a pointer, dereference it
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	// ensure that we have a struct
	if v.Kind() != reflect.Struct {
		return fmt.Sprintf("%v", argument)
	}

	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// get the json tag; if not set or if it's "-", fall back to the field name
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			jsonTag = field.Name
		}
		value := v.Field(i).Interface()
		output += fmt.Sprintf("%s=%v,", jsonTag, value)
	}

	// remove trailing comma if present
	if len(output) > 0 {
		output = output[:len(output)-1]
	}

	return output
}

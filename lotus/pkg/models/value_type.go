package models

import (
	"fmt"
	"reflect"
	"time"

	"github.com/Gobusters/ectolinq"
)

type ValueType string

const (
	ValueTypeString ValueType = "string"
	ValueTypeNumber ValueType = "number"
	ValueTypeBool   ValueType = "bool"
	ValueTypeArray  ValueType = "array"
	ValueTypeObject ValueType = "object"
	ValueTypeDate   ValueType = "date"
	ValueTypeAny    ValueType = "any"
)

type ActionValueType struct {
	Type  ValueType `json:"type" validate:"required,oneof=string int float bool array object date any"`
	Items ValueType `json:"items" validate:"omitempty,oneof=string int float bool array object date any"`
}

func (a *ActionValueType) ToString() string {
	if a.Items != "" {
		return fmt.Sprintf("[%s]", a.Items)
	}

	return string(a.Type)
}

func ValidateActionValueType(actualType, expectedType ActionValueType) error {
	if actualType.Type == "" {
		actualType.Type = "none"
	}

	// Empty or "any" as expected type allows any match
	if expectedType.Type == "" || expectedType.Type == ValueTypeAny || actualType.Type == ValueTypeAny {
		return nil
	}

	if actualType.Type != expectedType.Type {
		return fmt.Errorf("expected type %s but got %s", expectedType.Type, actualType.Type)
	}

	if expectedType.Type == ValueTypeArray {
		// Empty or "any" items type matches any items
		if expectedType.Items == ValueTypeAny || expectedType.Items == "" || actualType.Items == ValueTypeAny {
			return nil
		}

		if actualType.Items != expectedType.Items {
			return fmt.Errorf("expected type %s with items %s but got type %s with items %s", expectedType.Type, expectedType.Items, actualType.Type, actualType.Items)
		}
	}

	return nil
}

func IsActionValueType(value any, expectedType ActionValueType) error {
	// Allow nil values to pass through - they're handled at a higher level
	if value == nil {
		return nil
	}

	if expectedType.Type == ValueTypeArray {
		if !IsType(value, expectedType.Type) {
			return fmt.Errorf("expected type %s with items %s but got %s", expectedType.Type, expectedType.Items, value)
		}

		// Use reflection to iterate over any slice type ([]any, []string, etc.)
		if expectedType.Items != "" && expectedType.Items != ValueTypeAny {
			rv := reflect.ValueOf(value)
			for i := 0; i < rv.Len(); i++ {
				item := rv.Index(i).Interface()
				if !IsType(item, expectedType.Items) {
					return fmt.Errorf("expected type %s with items %s but got %s", expectedType.Type, expectedType.Items, value)
				}
			}
		}

		return nil
	}

	if !IsType(value, expectedType.Type) {
		return fmt.Errorf("expected type %s but got %T", expectedType.Type, value)
	}

	return nil
}

func IsType(value any, expectedType ValueType) bool {
	switch expectedType {
	case ValueTypeAny:
		return true
	case ValueTypeString:
		_, ok := value.(string)
		return ok
	case ValueTypeNumber:
		_, ok := value.(int)
		if ok {
			return true
		}

		_, ok = value.(float64)
		return ok
	case ValueTypeBool:
		_, ok := value.(bool)
		return ok
	case ValueTypeArray:
		// Check for []any first
		if _, ok := value.([]any); ok {
			return true
		}
		// Use reflection to check for any slice type ([]string, []int, etc.)
		rv := reflect.ValueOf(value)
		return rv.Kind() == reflect.Slice
	case ValueTypeObject:
		_, ok := value.(map[string]any)
		return ok
	case ValueTypeDate:
		_, ok := value.(time.Time)
		return ok
	}
	return false
}

func ValidateActionArguments(arguments []any, expectedTypes []ActionValueType) error {
	for i, expectedType := range expectedTypes {
		if err := IsActionValueType(ectolinq.At(arguments, i), expectedType); err != nil {
			return err
		}
	}

	return nil
}

func GetDefault(expectedType ActionValueType) any {
	switch expectedType.Type {
	case ValueTypeString:
		return ""
	case ValueTypeNumber:
		return 0.0
	case ValueTypeBool:
		return false
	case ValueTypeArray:
		return []any{}
	case ValueTypeObject:
		return map[string]any{}
	case ValueTypeDate:
		return time.Time{}
	}

	return nil
}

func GetActionValueType(input any) ActionValueType {
	switch value := input.(type) {
	case string:
		return ActionValueType{Type: ValueTypeString}
	case int:
		return ActionValueType{Type: ValueTypeNumber}
	case float64:
		return ActionValueType{Type: ValueTypeNumber}
	case bool:
		return ActionValueType{Type: ValueTypeBool}
	case []any:
		// get the type of the items
		if len(value) > 0 {
			itemsType := GetActionValueType(value[0])
			return ActionValueType{Type: ValueTypeArray, Items: itemsType.Type}
		}
		return ActionValueType{Type: ValueTypeArray, Items: ValueTypeAny}
	case map[string]any:
		return ActionValueType{Type: ValueTypeObject}
	case time.Time:
		return ActionValueType{Type: ValueTypeDate}
	}

	return ActionValueType{Type: ValueTypeAny}
}

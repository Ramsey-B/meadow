package utils

import (
	"fmt"
	"reflect"
)

func AnyToType[T any](input any) (T, error) {
	var zero T
	if input == nil {
		return zero, nil
	}

	// Try direct type assertion first
	if result, ok := input.(T); ok {
		return result, nil
	}

	// Handle numeric conversions using reflection
	targetType := reflect.TypeOf(zero)
	// If targetType is nil (T is an interface like 'any'), we can't do reflection conversion
	if targetType == nil {
		return zero, fmt.Errorf("type mismatch: expected %T, got %T", zero, input)
	}

	inputValue := reflect.ValueOf(input)

	// Special case: convert any slice type to []any
	if targetType == reflect.TypeOf([]any{}) && inputValue.Kind() == reflect.Slice {
		result := make([]any, inputValue.Len())
		for i := 0; i < inputValue.Len(); i++ {
			result[i] = inputValue.Index(i).Interface()
		}
		// Cast back to T (which is []any)
		if converted, ok := any(result).(T); ok {
			return converted, nil
		}
	}

	// Numeric conversions only (avoid surprising conversions like int -> string (rune)).
	if isNumericKind(inputValue.Kind()) && isNumericKind(targetType.Kind()) && inputValue.Type().ConvertibleTo(targetType) {
		converted := inputValue.Convert(targetType)
		if result, ok := converted.Interface().(T); ok {
			return result, nil
		}
	}

	return zero, fmt.Errorf("type mismatch: expected %T, got %T", zero, input)
}

func isNumericKind(k reflect.Kind) bool {
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

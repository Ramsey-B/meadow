package utils

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New(validator.WithRequiredStructEnabled())

func ParseArguments[T any](args any) (T, error) {
	var result T

	// check if args is already the correct type
	if arg, ok := args.(T); ok {
		return arg, nil
	}

	// Convert args to T via JSON marshalling/unmarshalling.
	// This assumes args is a type that can be marshaled to JSON and matches the structure of T.
	b, err := json.Marshal(args)
	if err != nil {
		return result, err
	}

	if err = json.Unmarshal(b, &result); err != nil {
		return result, fmt.Errorf("argument %v is not a valid %T", args, result)
	}

	return result, nil
}

func ValidateArguments[T any](args any) (T, error) {
	result, err := ParseArguments[T](args)
	if err != nil {
		return result, err
	}

	// Use go-playground/validator to validate the struct fields.
	if err = validate.Struct(result); err != nil {
		return result, ValidationErrorToString(result, err)
	}

	return result, nil
}

func Validate[T any](value T) (T, error) {
	if err := validate.Struct(value); err != nil {
		return value, ValidationErrorToString(value, err)
	}

	return value, nil
}

func ValidateValue(value any, tag string) error {
	err := validate.Var(value, tag)
	if err != nil {
		return ValidationErrorToString(value, err)
	}
	return nil
}

func ValidationErrorToString(input any, err error) error {
	// Check if the error is a ValidationErrors type
	if verrs, ok := err.(validator.ValidationErrors); ok {
		// Build a custom error message for each field error.
		msg := ""
		for _, fe := range verrs {
			// fe.Tag() contains the validation tag that failed (e.g., "min").
			// fe.Param() contains the parameter for that tag (e.g., "10").
			// fe.Value() contains the actual value provided.
			// fe.StructField() gives the field name.
			msg += fmt.Sprintf("\n • Failed %T validation for field '%s': rule '%s' expected '%s', got '%v'.", input, fe.StructField(), fe.Tag(), fe.Param(), fe.Value())
		}
		return errors.New(msg)
	}

	return err
}

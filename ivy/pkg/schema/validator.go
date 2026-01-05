package schema

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/Ramsey-B/ivy/pkg/models"
)

// ValidationError represents a single validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ValidationResult represents the result of validating entity data
type ValidationResult struct {
	Valid  bool              `json:"valid"`
	Errors []ValidationError `json:"errors,omitempty"`
}

// Validator validates entity data against a schema
type Validator struct {
	schema models.EntityTypeSchema
}

// NewValidator creates a new validator from a JSON schema
func NewValidator(schemaJSON json.RawMessage) (*Validator, error) {
	var schema models.EntityTypeSchema
	if err := json.Unmarshal(schemaJSON, &schema); err != nil {
		return nil, fmt.Errorf("failed to parse schema: %w", err)
	}
	return &Validator{schema: schema}, nil
}

// NewValidatorFromSchema creates a new validator from a parsed schema
func NewValidatorFromSchema(schema models.EntityTypeSchema) *Validator {
	return &Validator{schema: schema}
}

// Validate validates entity data against the schema
func (v *Validator) Validate(data map[string]any) ValidationResult {
	result := ValidationResult{Valid: true, Errors: []ValidationError{}}

	// Check required fields
	for _, required := range v.schema.Required {
		if _, exists := data[required]; !exists {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Field:   required,
				Message: "required field is missing",
			})
		}
	}

	// Validate each field against its definition
	for fieldName, fieldDef := range v.schema.Properties {
		value, exists := data[fieldName]
		if !exists {
			continue // Not required fields are allowed to be missing
		}

		if value == nil {
			// Null values are allowed unless field is required
			continue
		}

		fieldErrors := validateField(fieldName, value, fieldDef)
		if len(fieldErrors) > 0 {
			result.Valid = false
			result.Errors = append(result.Errors, fieldErrors...)
		}
	}

	// Check for unknown fields (optional - could be configurable)
	// for fieldName := range data {
	// 	if _, exists := v.schema.Properties[fieldName]; !exists {
	// 		result.Errors = append(result.Errors, ValidationError{
	// 			Field:   fieldName,
	// 			Message: "unknown field",
	// 		})
	// 	}
	// }

	return result
}

// validateField validates a single field value against its definition
func validateField(fieldName string, value any, def models.PropertyDefinition) []ValidationError {
	var errors []ValidationError

	// Type validation
	if !isValidType(value, def.Type) {
		errors = append(errors, ValidationError{
			Field:   fieldName,
			Message: fmt.Sprintf("expected type %s, got %s", def.Type, getTypeName(value)),
		})
		return errors // No point checking further if type is wrong
	}

	// Format validation
	if def.Format != "" {
		if err := validateFormat(value, def.Format); err != nil {
			errors = append(errors, ValidationError{
				Field:   fieldName,
				Message: err.Error(),
			})
		}
	}

	// Nested object validation
	if def.Type == "object" && def.Properties != nil {
		if objValue, ok := value.(map[string]any); ok {
			for nestedName, nestedDef := range def.Properties {
				if nestedValue, exists := objValue[nestedName]; exists && nestedValue != nil {
					nestedErrors := validateField(fieldName+"."+nestedName, nestedValue, nestedDef)
					errors = append(errors, nestedErrors...)
				}
			}
		}
	}

	// Array item validation
	if def.Type == "array" && def.Items != nil {
		if arrValue, ok := value.([]any); ok {
			for i, item := range arrValue {
				itemErrors := validateField(fmt.Sprintf("%s[%d]", fieldName, i), item, *def.Items)
				errors = append(errors, itemErrors...)
			}
		}
	}

	return errors
}

// isValidType checks if a value matches the expected JSON Schema type
func isValidType(value any, expectedType string) bool {
	if value == nil {
		return true // null is valid for any type (handled elsewhere for required)
	}

	switch expectedType {
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		// JSON numbers can be float64 or int
		switch value.(type) {
		case float64, float32, int, int64, int32:
			return true
		}
		return false
	case "integer":
		switch v := value.(type) {
		case float64:
			return v == float64(int64(v)) // Check if it's a whole number
		case int, int64, int32:
			return true
		}
		return false
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		rv := reflect.ValueOf(value)
		return rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array
	default:
		return true // Unknown types pass (permissive)
	}
}

// getTypeName returns the JSON type name for a Go value
func getTypeName(value any) string {
	if value == nil {
		return "null"
	}

	switch value.(type) {
	case string:
		return "string"
	case float64, float32, int, int64, int32:
		return "number"
	case bool:
		return "boolean"
	case map[string]any:
		return "object"
	case []any:
		return "array"
	default:
		rv := reflect.ValueOf(value)
		if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
			return "array"
		}
		return fmt.Sprintf("%T", value)
	}
}

// validateFormat validates a value against a format constraint
func validateFormat(value any, format string) error {
	str, ok := value.(string)
	if !ok {
		return nil // Format only applies to strings
	}

	switch format {
	case "email":
		if !isValidEmail(str) {
			return fmt.Errorf("invalid email format")
		}
	case "date":
		if !isValidDate(str) {
			return fmt.Errorf("invalid date format (expected YYYY-MM-DD)")
		}
	case "date-time":
		if !isValidDateTime(str) {
			return fmt.Errorf("invalid date-time format (expected ISO 8601)")
		}
	case "phone":
		if !isValidPhone(str) {
			return fmt.Errorf("invalid phone format")
		}
	case "uri", "url":
		if !isValidURI(str) {
			return fmt.Errorf("invalid URI format")
		}
	case "uuid":
		if !isValidUUID(str) {
			return fmt.Errorf("invalid UUID format")
		}
	// Add more formats as needed
	}

	return nil
}

// Format validation regexes
var (
	emailRegex    = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	dateRegex     = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	dateTimeRegex = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}(T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|[+-]\d{2}:\d{2})?)?$`)
	phoneRegex    = regexp.MustCompile(`^[\d\s\-\+\(\)]{7,20}$`)
	uriRegex      = regexp.MustCompile(`^(https?|ftp)://[^\s/$.?#].[^\s]*$`)
	uuidRegex     = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

func isValidEmail(s string) bool {
	return emailRegex.MatchString(s)
}

func isValidDate(s string) bool {
	return dateRegex.MatchString(s)
}

func isValidDateTime(s string) bool {
	return dateTimeRegex.MatchString(s)
}

func isValidPhone(s string) bool {
	// Remove common separators for validation
	cleaned := strings.ReplaceAll(s, " ", "")
	cleaned = strings.ReplaceAll(cleaned, "-", "")
	cleaned = strings.ReplaceAll(cleaned, "(", "")
	cleaned = strings.ReplaceAll(cleaned, ")", "")
	return len(cleaned) >= 7 && len(cleaned) <= 15
}

func isValidURI(s string) bool {
	return uriRegex.MatchString(s)
}

func isValidUUID(s string) bool {
	return uuidRegex.MatchString(s)
}


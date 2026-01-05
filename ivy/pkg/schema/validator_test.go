package schema

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidator_RequiredFields(t *testing.T) {
	schemaJSON := json.RawMessage(`{
		"properties": {
			"first_name": {"type": "string"},
			"last_name": {"type": "string"},
			"email": {"type": "string", "format": "email"}
		},
		"required": ["first_name", "last_name"]
	}`)

	validator, err := NewValidator(schemaJSON)
	require.NoError(t, err)

	t.Run("valid data with all required fields", func(t *testing.T) {
		data := map[string]any{
			"first_name": "John",
			"last_name":  "Doe",
		}
		result := validator.Validate(data)
		assert.True(t, result.Valid)
		assert.Empty(t, result.Errors)
	})

	t.Run("missing required field", func(t *testing.T) {
		data := map[string]any{
			"first_name": "John",
		}
		result := validator.Validate(data)
		assert.False(t, result.Valid)
		assert.Len(t, result.Errors, 1)
		assert.Equal(t, "last_name", result.Errors[0].Field)
	})

	t.Run("optional field can be missing", func(t *testing.T) {
		data := map[string]any{
			"first_name": "John",
			"last_name":  "Doe",
			// email is optional
		}
		result := validator.Validate(data)
		assert.True(t, result.Valid)
	})
}

func TestValidator_TypeValidation(t *testing.T) {
	schemaJSON := json.RawMessage(`{
		"properties": {
			"name": {"type": "string"},
			"age": {"type": "integer"},
			"score": {"type": "number"},
			"active": {"type": "boolean"},
			"tags": {"type": "array"},
			"meta": {"type": "object"}
		}
	}`)

	validator, err := NewValidator(schemaJSON)
	require.NoError(t, err)

	t.Run("valid types", func(t *testing.T) {
		data := map[string]any{
			"name":   "John",
			"age":    float64(30), // JSON numbers are float64
			"score":  95.5,
			"active": true,
			"tags":   []any{"a", "b"},
			"meta":   map[string]any{"key": "value"},
		}
		result := validator.Validate(data)
		assert.True(t, result.Valid)
	})

	t.Run("wrong type for string", func(t *testing.T) {
		data := map[string]any{
			"name": 123, // should be string
		}
		result := validator.Validate(data)
		assert.False(t, result.Valid)
		assert.Equal(t, "name", result.Errors[0].Field)
	})

	t.Run("wrong type for integer", func(t *testing.T) {
		data := map[string]any{
			"age": 30.5, // should be whole number
		}
		result := validator.Validate(data)
		assert.False(t, result.Valid)
	})

	t.Run("integer accepts whole float", func(t *testing.T) {
		data := map[string]any{
			"age": float64(30), // JSON serializes as float64
		}
		result := validator.Validate(data)
		assert.True(t, result.Valid)
	})
}

func TestValidator_FormatValidation(t *testing.T) {
	schemaJSON := json.RawMessage(`{
		"properties": {
			"email": {"type": "string", "format": "email"},
			"dob": {"type": "string", "format": "date"},
			"created_at": {"type": "string", "format": "date-time"},
			"website": {"type": "string", "format": "uri"},
			"id": {"type": "string", "format": "uuid"}
		}
	}`)

	validator, err := NewValidator(schemaJSON)
	require.NoError(t, err)

	t.Run("valid email", func(t *testing.T) {
		data := map[string]any{"email": "john@example.com"}
		result := validator.Validate(data)
		assert.True(t, result.Valid)
	})

	t.Run("invalid email", func(t *testing.T) {
		data := map[string]any{"email": "not-an-email"}
		result := validator.Validate(data)
		assert.False(t, result.Valid)
		assert.Contains(t, result.Errors[0].Message, "email")
	})

	t.Run("valid date", func(t *testing.T) {
		data := map[string]any{"dob": "1990-01-15"}
		result := validator.Validate(data)
		assert.True(t, result.Valid)
	})

	t.Run("invalid date", func(t *testing.T) {
		data := map[string]any{"dob": "01/15/1990"}
		result := validator.Validate(data)
		assert.False(t, result.Valid)
	})

	t.Run("valid datetime", func(t *testing.T) {
		data := map[string]any{"created_at": "2024-01-15T10:30:00Z"}
		result := validator.Validate(data)
		assert.True(t, result.Valid)
	})

	t.Run("valid uuid", func(t *testing.T) {
		data := map[string]any{"id": "550e8400-e29b-41d4-a716-446655440000"}
		result := validator.Validate(data)
		assert.True(t, result.Valid)
	})

	t.Run("invalid uuid", func(t *testing.T) {
		data := map[string]any{"id": "not-a-uuid"}
		result := validator.Validate(data)
		assert.False(t, result.Valid)
	})
}

func TestValidator_NestedObjects(t *testing.T) {
	schemaJSON := json.RawMessage(`{
		"properties": {
			"name": {
				"type": "object",
				"properties": {
					"first": {"type": "string"},
					"last": {"type": "string"}
				}
			},
			"address": {
				"type": "object",
				"properties": {
					"city": {"type": "string"},
					"zip": {"type": "string"}
				}
			}
		}
	}`)

	validator, err := NewValidator(schemaJSON)
	require.NoError(t, err)

	t.Run("valid nested object", func(t *testing.T) {
		data := map[string]any{
			"name": map[string]any{
				"first": "John",
				"last":  "Doe",
			},
		}
		result := validator.Validate(data)
		assert.True(t, result.Valid)
	})

	t.Run("invalid nested field type", func(t *testing.T) {
		data := map[string]any{
			"name": map[string]any{
				"first": 123, // should be string
				"last":  "Doe",
			},
		}
		result := validator.Validate(data)
		assert.False(t, result.Valid)
		assert.Equal(t, "name.first", result.Errors[0].Field)
	})
}

func TestValidator_Arrays(t *testing.T) {
	schemaJSON := json.RawMessage(`{
		"properties": {
			"tags": {
				"type": "array",
				"items": {"type": "string"}
			},
			"scores": {
				"type": "array",
				"items": {"type": "number"}
			}
		}
	}`)

	validator, err := NewValidator(schemaJSON)
	require.NoError(t, err)

	t.Run("valid string array", func(t *testing.T) {
		data := map[string]any{
			"tags": []any{"a", "b", "c"},
		}
		result := validator.Validate(data)
		assert.True(t, result.Valid)
	})

	t.Run("invalid array item type", func(t *testing.T) {
		data := map[string]any{
			"tags": []any{"a", 123, "c"}, // 123 should be string
		}
		result := validator.Validate(data)
		assert.False(t, result.Valid)
		assert.Equal(t, "tags[1]", result.Errors[0].Field)
	})
}

func TestValidator_NullValues(t *testing.T) {
	schemaJSON := json.RawMessage(`{
		"properties": {
			"name": {"type": "string"},
			"email": {"type": "string"}
		},
		"required": ["name"]
	}`)

	validator, err := NewValidator(schemaJSON)
	require.NoError(t, err)

	t.Run("null optional field is valid", func(t *testing.T) {
		data := map[string]any{
			"name":  "John",
			"email": nil,
		}
		result := validator.Validate(data)
		assert.True(t, result.Valid)
	})
}


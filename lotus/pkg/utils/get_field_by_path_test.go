package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetFieldByPath(t *testing.T) {
	t.Run("should return the value if the path is a single key", func(t *testing.T) {
		value := map[string]any{
			"key": "value",
		}

		result, err := GetFieldByPath(value, "key")
		assert.NoError(t, err)
		assert.True(t, result.IsValid())
		assert.Equal(t, "value", result.Interface())
	})

	t.Run("should return the value if the path is a nested key", func(t *testing.T) {
		value := map[string]any{
			"key": map[string]any{
				"nested": "value",
			},
		}

		result, err := GetFieldByPath(value, "key.nested")
		assert.NoError(t, err)
		assert.True(t, result.IsValid())
		assert.Equal(t, "value", result.Interface())
	})

	t.Run("should return an error if the path is invalid", func(t *testing.T) {
		value := map[string]any{}

		_, err := GetFieldByPath(value, "invalid")
		assert.Error(t, err)
	})

	t.Run("should treat wildcard index as aggregation (items[*].id)", func(t *testing.T) {
		value := map[string]any{
			"items": []any{
				map[string]any{"id": "a"},
				map[string]any{"id": "b"},
			},
		}

		result, err := GetFieldByPath(value, "items[*].id")
		assert.NoError(t, err)
		assert.True(t, result.IsValid())
		assert.Equal(t, []string{"a", "b"}, result.Interface())
	})
}

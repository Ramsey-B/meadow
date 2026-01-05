package stagedentity

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeepMergeJSON(t *testing.T) {
	tests := []struct {
		name     string
		target   string
		source   string
		expected string
	}{
		{
			name:     "simple merge - add new field",
			target:   `{"name": "John", "age": 30}`,
			source:   `{"email": "john@example.com"}`,
			expected: `{"name": "John", "age": 30, "email": "john@example.com"}`,
		},
		{
			name:     "simple merge - overwrite field",
			target:   `{"name": "John", "age": 30}`,
			source:   `{"age": 31}`,
			expected: `{"name": "John", "age": 31}`,
		},
		{
			name:     "nested merge - preserve target nested fields",
			target:   `{"user": {"name": "John", "age": 30}}`,
			source:   `{"user": {"email": "john@example.com"}}`,
			expected: `{"user": {"name": "John", "age": 30, "email": "john@example.com"}}`,
		},
		{
			name:     "nested merge - overwrite nested field",
			target:   `{"user": {"name": "John", "age": 30}}`,
			source:   `{"user": {"age": 31}}`,
			expected: `{"user": {"name": "John", "age": 31}}`,
		},
		{
			name:     "deep nested merge",
			target:   `{"user": {"profile": {"name": "John", "bio": "Developer"}}}`,
			source:   `{"user": {"profile": {"age": 30}, "email": "john@example.com"}}`,
			expected: `{"user": {"profile": {"name": "John", "bio": "Developer", "age": 30}, "email": "john@example.com"}}`,
		},
		{
			name:     "array replacement - arrays are not merged",
			target:   `{"tags": ["a", "b"]}`,
			source:   `{"tags": ["c", "d"]}`,
			expected: `{"tags": ["c", "d"]}`,
		},
		{
			name:     "mixed types - source overwrites",
			target:   `{"data": {"field": "string"}}`,
			source:   `{"data": 123}`,
			expected: `{"data": 123}`,
		},
		{
			name:     "complex nested structure",
			target:   `{"person": {"name": "John", "addresses": [{"city": "NYC"}]}, "metadata": {"created": "2024-01-01"}}`,
			source:   `{"person": {"email": "john@example.com", "addresses": [{"city": "LA"}]}, "metadata": {"updated": "2024-01-02"}}`,
			expected: `{"person": {"name": "John", "email": "john@example.com", "addresses": [{"city": "LA"}]}, "metadata": {"created": "2024-01-01", "updated": "2024-01-02"}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := json.RawMessage(tt.target)
			source := json.RawMessage(tt.source)

			result, err := deepMergeJSON(target, source)
			require.NoError(t, err)

			// Parse both result and expected to compare as maps (to ignore key ordering)
			var resultMap map[string]any
			var expectedMap map[string]any

			err = json.Unmarshal(result, &resultMap)
			require.NoError(t, err)

			err = json.Unmarshal([]byte(tt.expected), &expectedMap)
			require.NoError(t, err)

			assert.Equal(t, expectedMap, resultMap)
		})
	}
}

func TestDeepMergeJSON_InvalidJSON(t *testing.T) {
	tests := []struct {
		name   string
		target string
		source string
	}{
		{
			name:   "invalid target JSON",
			target: `{invalid}`,
			source: `{"name": "John"}`,
		},
		{
			name:   "invalid source JSON",
			target: `{"name": "John"}`,
			source: `{invalid}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := json.RawMessage(tt.target)
			source := json.RawMessage(tt.source)

			_, err := deepMergeJSON(target, source)
			assert.Error(t, err)
		})
	}
}

func TestDeepMerge(t *testing.T) {
	tests := []struct {
		name     string
		target   map[string]any
		source   map[string]any
		expected map[string]any
	}{
		{
			name:     "add new top-level field",
			target:   map[string]any{"a": 1},
			source:   map[string]any{"b": 2},
			expected: map[string]any{"a": 1, "b": 2},
		},
		{
			name:     "overwrite top-level field",
			target:   map[string]any{"a": 1},
			source:   map[string]any{"a": 2},
			expected: map[string]any{"a": 2},
		},
		{
			name: "merge nested maps",
			target: map[string]any{
				"user": map[string]any{"name": "John"},
			},
			source: map[string]any{
				"user": map[string]any{"age": 30},
			},
			expected: map[string]any{
				"user": map[string]any{"name": "John", "age": 30},
			},
		},
		{
			name: "overwrite with non-map value",
			target: map[string]any{
				"data": map[string]any{"nested": "value"},
			},
			source: map[string]any{
				"data": "string",
			},
			expected: map[string]any{
				"data": "string",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deepMerge(tt.target, tt.source)
			assert.Equal(t, tt.expected, tt.target)
		})
	}
}

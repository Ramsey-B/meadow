package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAssignMapValue(t *testing.T) {
	t.Run("should assign value to map", func(t *testing.T) {
		targetRaw := map[string]any{}
		result := AssignMapValue(targetRaw, "foo.bar", "test")

		foo, ok := result["foo"].(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, "test", foo["bar"])
	})

	t.Run("should assign value to map with nested map", func(t *testing.T) {
		targetRaw := map[string]any{}
		result := AssignMapValue(targetRaw, "foo.bar.baz", "test")

		foo, ok := result["foo"].(map[string]any)
		assert.True(t, ok)

		bar, ok := foo["bar"].(map[string]any)
		assert.True(t, ok)

		assert.Equal(t, "test", bar["baz"])
	})

	t.Run("should return unmodified map if path is empty", func(t *testing.T) {
		targetRaw := map[string]any{}
		result := AssignMapValue(targetRaw, "", "test")
		assert.Equal(t, targetRaw, result)
	})

	t.Run("should override existing value if path already exists", func(t *testing.T) {
		targetRaw := map[string]any{}
		targetRaw["foo"] = "bar"
		result := AssignMapValue(targetRaw, "foo", "test")
		assert.Equal(t, "test", result["foo"])
	})
}

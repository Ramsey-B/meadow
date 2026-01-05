package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetActionValueType(t *testing.T) {

	t.Run("string", func(t *testing.T) {
		actual := GetActionValueType("test")
		expected := ActionValueType{Type: ValueTypeString}
		assert.Equal(t, expected, actual)
	})

	t.Run("int", func(t *testing.T) {
		actual := GetActionValueType(1)
		expected := ActionValueType{Type: ValueTypeNumber}
		assert.Equal(t, expected, actual)
	})

	t.Run("float", func(t *testing.T) {
		actual := GetActionValueType(1.0)
		expected := ActionValueType{Type: ValueTypeNumber}
		assert.Equal(t, expected, actual)
	})

	t.Run("bool", func(t *testing.T) {
		actual := GetActionValueType(true)
		expected := ActionValueType{Type: ValueTypeBool}
		assert.Equal(t, expected, actual)

		actual = GetActionValueType(false)
		expected = ActionValueType{Type: ValueTypeBool}
		assert.Equal(t, expected, actual)
	})

	t.Run("array", func(t *testing.T) {
		actual := GetActionValueType([]any{1, 2, 3})
		expected := ActionValueType{Type: ValueTypeArray, Items: ValueTypeNumber}
		assert.Equal(t, expected, actual)

		actual = GetActionValueType([]any{"test", "test2", "test3"})
		expected = ActionValueType{Type: ValueTypeArray, Items: ValueTypeString}
		assert.Equal(t, expected, actual)

		actual = GetActionValueType([]any{true, false, true})
		expected = ActionValueType{Type: ValueTypeArray, Items: ValueTypeBool}
		assert.Equal(t, expected, actual)

		actual = GetActionValueType([]any{time.Now(), time.Now(), time.Now()})
		expected = ActionValueType{Type: ValueTypeArray, Items: ValueTypeDate}
		assert.Equal(t, expected, actual)

		actual = GetActionValueType([]any{map[string]any{"test": "test"}, map[string]any{"test": "test2"}, map[string]any{"test": "test3"}})
		expected = ActionValueType{Type: ValueTypeArray, Items: ValueTypeObject}
		assert.Equal(t, expected, actual)

		actual = GetActionValueType([]any{[]any{1}})
		expected = ActionValueType{Type: ValueTypeArray, Items: ValueTypeArray}
		assert.Equal(t, expected, actual)

		actual = GetActionValueType([]any{})
		expected = ActionValueType{Type: ValueTypeArray, Items: ValueTypeAny}
		assert.Equal(t, expected, actual)
	})

	t.Run("object", func(t *testing.T) {
		actual := GetActionValueType(map[string]any{"test": "test"})
		expected := ActionValueType{Type: ValueTypeObject}
		assert.Equal(t, expected, actual)
	})

	t.Run("date", func(t *testing.T) {
		actual := GetActionValueType(time.Now())
		expected := ActionValueType{Type: ValueTypeDate}
		assert.Equal(t, expected, actual)
	})

	t.Run("any", func(t *testing.T) {
		actual := GetActionValueType(nil)
		expected := ActionValueType{Type: ValueTypeAny}
		assert.Equal(t, expected, actual)
	})
}

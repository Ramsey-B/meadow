package text

import (
	"testing"

	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestSplitAction(t *testing.T) {
	t.Run("should return error if input is not a string", func(t *testing.T) {
		args := TextSplitArguments{
			Separator: ",",
		}

		action, err := NewTextSplitAction("split", args, models.ActionValueType{
			Type: models.ValueTypeString,
		})
		assert.NoError(t, err)

		result, err := action.Execute(1)
		assert.Error(t, err)
		assert.Equal(t, "action 'split': expected at least 1 string inputs for text argument, got 0", err.Error())
		assert.Zero(t, result)
	})

	t.Run("should return error if separator is not a string", func(t *testing.T) {
		args := map[string]any{
			"separator": 1,
		}

		_, err := NewTextSplitAction("split", args, models.ActionValueType{
			Type: models.ValueTypeString,
		})
		assert.Error(t, err)
		assert.Equal(t, "action 'split': argument map[separator:1] is not a valid text.TextSplitArguments", err.Error())
	})

	t.Run("should split string by separator", func(t *testing.T) {
		args := TextSplitArguments{
			Separator: ",",
		}

		action, err := NewTextSplitAction("split", args, models.ActionValueType{
			Type: models.ValueTypeString,
		})
		assert.NoError(t, err)

		result, err := action.Execute("test,test,test")
		assert.NoError(t, err)
		assert.Equal(t, []any{"test", "test", "test"}, result)
	})

	t.Run("should split string by empty separator", func(t *testing.T) {
		args := TextSplitArguments{
			Separator: "",
		}

		action, err := NewTextSplitAction("split", args, models.ActionValueType{
			Type: models.ValueTypeString,
		})
		assert.NoError(t, err)

		result, err := action.Execute("apple")
		assert.NoError(t, err)
		assert.Equal(t, []any{"a", "p", "p", "l", "e"}, result)
	})
}

package text

import (
	"testing"

	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestTextMinLength(t *testing.T) {
	t.Run("should return false if text is under min length", func(t *testing.T) {
		input := "foo bar"

		args := TextMinLengthArguments{
			MinLength: 8,
		}

		action, err := NewTextMinLengthAction("min-lenth", args, models.ActionValueType{
			Type: models.ValueTypeString,
		})
		assert.NoError(t, err)

		output, err := action.Execute(input)
		assert.NoError(t, err)
		assert.Equal(t, false, output)
	})

	t.Run("should return true if text is under min length", func(t *testing.T) {
		input := "foo bar"

		args := TextMinLengthArguments{
			MinLength: 7,
		}

		action, err := NewTextMinLengthAction("min-lenth", args, models.ActionValueType{
			Type: models.ValueTypeString,
		})
		assert.NoError(t, err)

		output, err := action.Execute(input)
		assert.NoError(t, err)
		assert.Equal(t, true, output)
	})
}

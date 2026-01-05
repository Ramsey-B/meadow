package number

import (
	"testing"

	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestNumberRoundAction_Execute(t *testing.T) {

	tests := []struct {
		name     string
		input    any
		args     any
		expected any
	}{
		{
			name:     "round to 0 decimal places",
			input:    12.345,
			args:     NumberRoundArguments{Precision: 0},
			expected: 12.0,
		},
		{
			name:     "round to 1 decimal place",
			input:    12.345,
			args:     NumberRoundArguments{Precision: 1},
			expected: 12.3,
		},
		{
			name:     "round to 2 decimal places",
			input:    12.345,
			args:     NumberRoundArguments{Precision: 2},
			expected: 12.35,
		},
		{
			name:     "round to 3 decimal places",
			input:    12.345,
			args:     NumberRoundArguments{Precision: 3},
			expected: 12.345,
		},
		{
			name:     "round to 4 decimal places",
			input:    12.345,
			args:     NumberRoundArguments{Precision: 4},
			expected: 12.345,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			action, err := NewNumberRoundAction("number_round", test.args, models.ActionValueType{
				Type: models.ValueTypeNumber,
			})
			assert.NoError(t, err)

			result, err := action.Execute(test.input, test.args)
			assert.NoError(t, err)
			assert.Equal(t, test.expected, result)
		})
	}
}

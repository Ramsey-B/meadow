package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAnyToType(t *testing.T) {
	type testCase struct {
		name     string
		input    any
		expected any
		err      bool
	}

	intTestCases := []testCase{
		{name: "int", input: 1, expected: 1, err: false},
		{name: "int string", input: "1", expected: 1, err: true},
		{name: "int float", input: 1.0, expected: 1, err: false},
		{name: "int bool", input: true, expected: 1, err: true},
		{name: "int slice", input: []any{1, 2, 3}, expected: 1, err: true},
		{name: "int time", input: time.Now(), expected: 1, err: true},
	}

	for _, testCase := range intTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := AnyToType[int](testCase.input)
			if testCase.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.expected, result)
			}
		})
	}

	stringTestCases := []testCase{
		{name: "string", input: "test", expected: "test", err: false},
		{name: "string int", input: 1, expected: "1", err: true},
		{name: "string float", input: 1.0, expected: "1.0", err: true},
		{name: "string bool", input: true, expected: "true", err: true},
		{name: "string slice", input: []any{1, 2, 3}, expected: "1,2,3", err: true},
		{name: "string time", input: time.Now(), expected: time.Now().Format(time.RFC3339), err: true},
	}

	for _, testCase := range stringTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := AnyToType[string](testCase.input)
			if testCase.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.expected, result)
			}
		})
	}

	floatTestCases := []testCase{
		{name: "float", input: 1.0, expected: 1.0, err: false},
		{name: "float string", input: "1.0", expected: 1.0, err: true},
		{name: "float int", input: 1, expected: 1.0, err: false},
		{name: "float bool", input: true, expected: 1.0, err: true},
		{name: "float slice", input: []any{1, 2, 3}, expected: 1.0, err: true},
		{name: "float time", input: time.Now(), expected: 1.0, err: true},
	}

	for _, testCase := range floatTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := AnyToType[float64](testCase.input)
			if testCase.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.expected, result)
			}
		})
	}

	boolTestCases := []testCase{
		{name: "bool", input: true, expected: true, err: false},
		{name: "bool string", input: "true", expected: true, err: true},
		{name: "bool int", input: 1, expected: true, err: true},
		{name: "bool float", input: 1.0, expected: true, err: true},
		{name: "bool slice", input: []any{1, 2, 3}, expected: true, err: true},
		{name: "bool time", input: time.Now(), expected: true, err: true},
	}

	for _, testCase := range boolTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := AnyToType[bool](testCase.input)
			if testCase.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.expected, result)
			}
		})
	}

	sliceTestCases := []testCase{
		{name: "slice", input: []any{1, 2, 3}, expected: []any{1, 2, 3}, err: false},
		{name: "slice string", input: "1,2,3", expected: []any{"1", "2", "3"}, err: true},
		{name: "slice float", input: 1.0, expected: []any{1.0}, err: true},
		{name: "slice bool", input: true, expected: []any{true}, err: true},
		{name: "slice time", input: time.Now(), expected: []any{time.Now()}, err: true},
	}

	for _, testCase := range sliceTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := AnyToType[[]any](testCase.input)
			if testCase.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.expected, result)
			}
		})
	}

	now := time.Now()
	timeTestCases := []testCase{
		{name: "time", input: now, expected: now, err: false},
		{name: "time string", input: now.Format(time.RFC3339), expected: now, err: true},
		{name: "time int", input: 1, expected: now, err: true},
		{name: "time float", input: 1.0, expected: now, err: true},
		{name: "time bool", input: true, expected: now, err: true},
		{name: "time slice", input: []any{1, 2, 3}, expected: now, err: true},
	}

	for _, testCase := range timeTestCases {
		t.Run(testCase.name, func(t *testing.T) {
			result, err := AnyToType[time.Time](testCase.input)
			if testCase.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, testCase.expected, result)
			}
		})
	}
}

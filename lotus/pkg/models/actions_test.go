package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateInputTypes(t *testing.T) {
	type testCase struct {
		rules         ActionInputRules
		inputs        []ActionValueType
		expectFailure bool
		testName      string
		testInputs    []any
		expectInputs  ActionInputs
	}

	testCases := []testCase{
		{
			rules: ActionInputRules{
				"input1": {
					Type: ValueTypeString,
					Min:  1,
					Max:  1,
				},
			},
			inputs: []ActionValueType{
				{Type: ValueTypeString},
			},
			expectFailure: false,
			testName:      "should handle single string input - pass",
			testInputs:    []any{"test"},
			expectInputs:  ActionInputs{"input1": {Value: []any{"test"}}},
		},
		{
			rules: ActionInputRules{
				"input1": {
					Type: ValueTypeString,
					Min:  1,
					Max:  1,
				},
			},
			inputs: []ActionValueType{
				{Type: ValueTypeString},
				{Type: ValueTypeNumber},
			},
			expectFailure: true,
			testName:      "should handle single string input - fail",
		},
		{
			rules: ActionInputRules{
				"input1": {
					Type: ValueTypeArray,
					Min:  1,
					Max:  1,
					Items: &ActionInputRule{
						Min: 1,
						Max: -1,
					},
					IsItemTypeDynamic: true,
				},
			},
			inputs: []ActionValueType{
				{Type: ValueTypeArray, Items: ValueTypeString},
				{Type: ValueTypeString},
			},
			expectFailure: false,
			testName:      "should handle dynamic array input - pass",
			testInputs:    []any{[]any{"test"}, "test"},
			expectInputs:  ActionInputs{"input1": {Value: []any{[]any{"test"}}, Items: []any{"test"}}},
		},
		{
			rules: ActionInputRules{
				"input1": {
					Type: ValueTypeArray,
					Min:  1,
					Max:  1,
					Items: &ActionInputRule{
						Min: 1,
						Max: -1,
					},
					IsItemTypeDynamic: true,
				},
			},
			inputs: []ActionValueType{
				{Type: ValueTypeArray, Items: ValueTypeString},
				{Type: ValueTypeNumber},
			},
			expectFailure: true,
			testName:      "should handle dynamic array input - fail",
		},
		{
			rules: ActionInputRules{
				"input1": {
					Type: ValueTypeString,
					Min:  1,
					Max:  1,
				},
				"input2": {
					Type: ValueTypeNumber,
					Min:  1,
					Max:  1,
				},
			},
			inputs: []ActionValueType{
				{Type: ValueTypeString},
				{Type: ValueTypeNumber},
			},
			expectFailure: false,
			testName:      "should handle multiple inputs - pass",
			testInputs:    []any{"test", 1},
			expectInputs:  ActionInputs{"input1": {Value: []any{"test"}}, "input2": {Value: []any{1}}},
		},
		{
			rules: ActionInputRules{
				"input1": {
					Type: ValueTypeString,
					Min:  1,
					Max:  1,
				},
				"input2": {
					Type: ValueTypeNumber,
					Min:  1,
					Max:  1,
				},
			},
			inputs: []ActionValueType{
				{Type: ValueTypeString},
				{Type: ValueTypeBool},
			},
			expectFailure: true,
			testName:      "should handle multiple inputs - fail",
		},
		{
			rules: ActionInputRules{
				"input1": {
					Type: ValueTypeString,
					Min:  1,
					Max:  1,
				},
			},
			inputs:        []ActionValueType{},
			expectFailure: true,
			testName:      "should handle no inputs - fail",
		},
		{
			rules: ActionInputRules{
				"input1": {
					Type: ValueTypeArray,
					Min:  1,
					Max:  -1,
				},
			},
			inputs: []ActionValueType{
				{Type: ValueTypeArray, Items: ValueTypeString},
				{Type: ValueTypeArray, Items: ValueTypeNumber},
			},
			expectFailure: true,
			testName:      "should handle multiple arrays with different items types - fail",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.testName, func(t *testing.T) {
			_, err := ValidateInputTypes(testCase.rules, testCase.inputs...)
			if testCase.expectFailure && err == nil {
				assert.Fail(t, "expected error, got nil")
			}

			if !testCase.expectFailure && err != nil {
				assert.Fail(t, "expected no error, got %v", err)
			}

			if !testCase.expectFailure {
				inputs, err := testCase.rules.Validate(testCase.testInputs...)
				assert.NoError(t, err)

				assert.Equal(t, testCase.expectInputs, inputs)
			}
		})
	}
}

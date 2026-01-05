package models

import (
	"github.com/Gobusters/ectolinq"
	"github.com/Ramsey-B/lotus/pkg/errors"
)

// ActionDefinition specifies which action to use in a step.
//
// Example:
//
//	{
//	  "key": "text_to_upper",  // The action's unique identifier
//	  "arguments": {},         // Static arguments (varies by action)
//	  "invert": false          // For bool-returning actions: flip true/false
//	}
//
// With arguments (e.g., for number_multiply):
//
//	{
//	  "key": "number_multiply",
//	  "arguments": {"value": 10}  // Multiplier value
//	}
type ActionDefinition struct {
	Key       string `json:"key" validate:"required"`      // Action identifier (e.g., "text_to_upper")
	Arguments any    `json:"arguments" validate:"omitempty"` // Static arguments for the action
	Invert    bool   `json:"invert" validate:"omitempty"`   // Invert boolean result (for validators/conditions)
}

// Action is the interface for all transformation/validation actions.
// Actions are registered in the registry and looked up by key.
type Action interface {
	GetKey() string                           // Unique action identifier
	GetInputType() ActionValueType            // Primary input type
	GetOutputType() ActionValueType           // Output type
	Execute(inputs ...any) (any, error)       // Execute the action
	GetInputRules() ActionInputRules          // Input validation rules
}

// ActionInputRules defines what inputs an action accepts.
// Keyed by input name (e.g., "value", "items").
type ActionInputRules map[string]ActionInputRule

// GetInputRules returns all rules as a slice.
func (r ActionInputRules) GetInputRules() []ActionInputRule {
	return ectolinq.Values(r)
}

// ActionInputRule defines constraints for a single input.
//
// Example: An action that accepts 1-2 numbers:
//
//	ActionInputRule{Type: ValueTypeNumber, Min: 1, Max: 2}
//
// Example: An action that accepts an array of any type:
//
//	ActionInputRule{
//	  Type: ValueTypeArray,
//	  Min: 1, Max: 1,
//	  Items: &ActionInputRule{Type: ValueTypeAny},
//	  IsItemTypeDynamic: true,
//	}
type ActionInputRule struct {
	Type              ValueType        `json:"type"`                // Expected value type
	Min               int              `json:"min"`                 // Minimum count (-1 = no min)
	Max               int              `json:"max"`                 // Maximum count (-1 = no max)
	Items             *ActionInputRule `json:"items"`               // For arrays: item type rule
	IsItemTypeDynamic bool             `json:"is_item_type_dynamic"` // Infer item type from first input
	CanBeArgument     bool             `json:"can_be_argument"`     // Can be provided via Arguments
}

func ValidateInputTypes(rules ActionInputRules, inputs ...ActionValueType) (ActionInputRules, error) {
	allowedTypes := []ValueType{}
	for _, rule := range rules {
		ruleTypes, err := validateInputType(rule, inputs...)
		if err != nil {
			return nil, err
		}

		allowedTypes = append(allowedTypes, ruleTypes...)
	}

	// check if any input type is not in the allowed types
	// If "any" is in allowed types, all types are allowed
	allTypesAllowed := ectolinq.Contains(allowedTypes, ValueTypeAny)
	if !allTypesAllowed {
		for _, input := range inputs {
			if !ectolinq.Contains(allowedTypes, input.Type) {
				return nil, errors.NewMappingErrorf("input type %s is not allowed", input.Type)
			}
		}
	}

	return rules, nil
}

func validateInputType(rule ActionInputRule, inputs ...ActionValueType) ([]ValueType, error) {
	allowedTypes := []ValueType{
		rule.Type,
	}

	// Match inputs by type: "any" matches everything
	matches := ectolinq.Filter(inputs, func(input ActionValueType) bool {
		return rule.Type == ValueTypeAny || rule.Type == input.Type
	})

	if len(matches) < rule.Min {
		return nil, errors.NewMappingErrorf("expected at least %d %s inputs, got %d", rule.Min, rule.Type, len(matches))
	}

	if len(matches) > rule.Max && rule.Max != -1 {
		return nil, errors.NewMappingErrorf("expected at most %d %s inputs, got %d", rule.Max, rule.Type, len(matches))
	}

	if rule.Items != nil {
		if rule.IsItemTypeDynamic && len(matches) > 0 {
			rule.Items.Type = matches[0].Items
			// If the matched array has no item type, default to "any"
			if rule.Items.Type == "" {
				rule.Items.Type = ValueTypeAny
			}
		}

		_, err := validateInputType(*rule.Items, inputs...)
		if err != nil {
			return nil, err
		}

		allowedTypes = append(allowedTypes, rule.Items.Type)
	} else if rule.Type == ValueTypeArray {
		// ensure that all the arrays have the same items type
		itemsType := ectolinq.First(matches).Items
		for _, match := range matches {
			if match.Items != itemsType {
				return nil, errors.NewMappingErrorf("all arrays must have the same items type, expected %s, got %s", itemsType, match.Items)
			}
		}

		allowedTypes = append(allowedTypes, itemsType)
	}

	return allowedTypes, nil
}

type ActionInputs map[string]ActionInput

type ActionInput struct {
	Value []any
	Items []any
}

func (r ActionInputRules) Validate(inputs ...any) (ActionInputs, error) {
	result := make(ActionInputs)

	for key, rule := range r {
		input := ActionInput{}
		valueType := ActionValueType{Type: rule.Type}

		if rule.Items != nil {
			valueType.Items = rule.Items.Type
		}

		for _, val := range inputs {
			// Check if value matches the main type
			matchesValue := IsActionValueType(val, valueType) == nil
			if matchesValue {
				input.Value = append(input.Value, val)
			}

			// Only add to Items if it didn't already match Value (to avoid double-counting)
			if rule.Items != nil && !matchesValue {
				if err := IsActionValueType(val, ActionValueType{Type: rule.Items.Type}); err == nil {
					input.Items = append(input.Items, val)
				}
			}
		}

		if rule.Min > len(input.Value) {
			return nil, errors.NewMappingErrorf("expected at least %d %s inputs for %s argument, got %d", rule.Min, rule.Type, key, len(input.Value))
		}

		if rule.Max > 0 && rule.Max < len(input.Value) {
			return nil, errors.NewMappingErrorf("expected at most %d %s inputs for %s argument, got %d", rule.Max, rule.Type, key, len(input.Value))
		}

		result[key] = input
	}

	return result, nil
}

package array

import (
	"github.com/Gobusters/ectolinq"
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var ArrayLengthRules = models.ActionInputRules{
	"array": {
		Type: models.ValueTypeArray,
		Min:  1,
		Max:  1,
	},
}

func NewArrayLengthAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(ArrayLengthRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	return &ArrayLengthAction{
		inputType: inputTypes[0],
		key:       key,
		rules:     rules,
	}, nil
}

type ArrayLengthAction struct {
	inputType models.ActionValueType
	key       string
	rules     models.ActionInputRules
}

func (a *ArrayLengthAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *ArrayLengthAction) GetKey() string {
	return a.key
}

func (a *ArrayLengthAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *ArrayLengthAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{
		Type: models.ValueTypeNumber,
	}
}

func (a *ArrayLengthAction) ValidateInputTypes(inputTypes ...models.ActionValueType) error {
	if len(inputTypes) != 1 {
		return errors.NewMappingErrorf("array length action requires 1 input, got %d", len(inputTypes))
	}

	// validate the input type is an array
	if inputTypes[0].Type != models.ValueTypeArray {
		return errors.NewMappingErrorf("array length action requires an array input, got %s", inputTypes[0].Type)
	}

	a.inputType = inputTypes[0]
	return nil
}

func (a *ArrayLengthAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, err
	}

	arrArgs := ectolinq.First(actionInputs["array"].Value)
	arr, err := utils.AnyToType[[]any](arrArgs)
	if err != nil {
		return nil, err
	}

	return len(arr), nil
}

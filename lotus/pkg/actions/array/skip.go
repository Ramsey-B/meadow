package array

import (
	"github.com/Gobusters/ectolinq"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var ArraySkipRules = models.ActionInputRules{
	"array": {
		Type: models.ValueTypeArray,
		Min:  1,
		Max:  1,
	},
	"count": {
		Type: models.ValueTypeNumber,
		Min:  0,
		Max:  1,
	},
}

type ArraySkipArguments struct {
	Count int `json:"count" validate:"omitempty,min=0"`
}

func NewArraySkipAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(ArraySkipRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ParseArguments[ArraySkipArguments](args)
	if err != nil {
		return nil, err
	}

	// Use the first input type directly to preserve items type
	var inputType models.ActionValueType
	if len(inputTypes) > 0 {
		inputType = inputTypes[0]
	}

	return &ArraySkipAction{
		count:     parsedArgs.Count,
		inputType: inputType,
		key:       key,
		rules:     rules,
	}, nil
}

type ArraySkipAction struct {
	inputType models.ActionValueType
	key       string
	count     int
	rules     models.ActionInputRules
}

func (a *ArraySkipAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *ArraySkipAction) GetKey() string {
	return a.key
}

func (a *ArraySkipAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *ArraySkipAction) GetOutputType() models.ActionValueType {
	return a.inputType
}

func (a *ArraySkipAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, err
	}

	arrArgs := ectolinq.First(actionInputs["array"].Value)
	arr, err := utils.AnyToType[[]any](arrArgs)
	if err != nil {
		return nil, err
	}

	// Use argument count first, fall back to input count
	count := a.count
	if count == 0 {
		if countInput, ok := actionInputs["count"]; ok && len(countInput.Value) > 0 {
			countArgs := ectolinq.First(countInput.Value)
			if countArgs != nil {
				inputCount, err := utils.AnyToType[int](countArgs)
				if err == nil {
					count = inputCount
				}
			}
		}
	}

	// Bounds check to prevent panic
	if count > len(arr) {
		count = len(arr)
	}
	if count < 0 {
		count = 0
	}

	return arr[count:], nil
}

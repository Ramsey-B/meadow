package array

import (
	"github.com/Gobusters/ectolinq"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var ArrayReverseRules = models.ActionInputRules{
	"array": {
		Type: models.ValueTypeArray,
		Min:  1,
		Max:  1,
	},
}

func NewArrayReverseAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(ArrayReverseRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	return &ArrayReverseAction{
		inputType: inputTypes[0],
		key:       key,
		rules:     rules,
	}, nil
}

type ArrayReverseAction struct {
	inputType models.ActionValueType
	key       string
	rules     models.ActionInputRules
}

func (a *ArrayReverseAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *ArrayReverseAction) GetKey() string {
	return a.key
}

func (a *ArrayReverseAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *ArrayReverseAction) GetOutputType() models.ActionValueType {
	return a.inputType
}

func (a *ArrayReverseAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, err
	}

	arrArgs := ectolinq.First(actionInputs["array"].Value)
	arr, err := utils.AnyToType[[]any](arrArgs)
	if err != nil {
		return nil, err
	}

	reverse := make([]any, len(arr))
	for i, v := range arr {
		reverse[len(arr)-i-1] = v
	}

	return reverse, nil
}

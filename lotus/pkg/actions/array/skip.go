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

	return &ArraySkipAction{
		count: parsedArgs.Count,
		inputType: models.ActionValueType{
			Type:  rules["array"].Type,
			Items: rules["array"].Items.Type,
		},
		key:   key,
		rules: rules,
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

	countArgs := ectolinq.First(actionInputs["count"].Value)
	count, err := utils.AnyToType[int](countArgs)
	if err != nil {
		return nil, err
	}

	if a.count > 0 {
		count = a.count
	}

	return arr[count:], nil
}

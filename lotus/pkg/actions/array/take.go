package array

import (
	"github.com/Gobusters/ectolinq"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var ArrayTakeRules = models.ActionInputRules{
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

type ArrayTakeArguments struct {
	Count int `json:"count" validate:"omitempty,min=0"`
}

func NewArrayTakeAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(ArrayTakeRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ParseArguments[ArrayTakeArguments](args)
	if err != nil {
		return nil, err
	}

	return &ArrayTakeAction{
		count: parsedArgs.Count,
		inputType: models.ActionValueType{
			Type:  rules["array"].Type,
			Items: rules["array"].Items.Type,
		},
		key:   key,
		rules: rules,
	}, nil
}

type ArrayTakeAction struct {
	inputType models.ActionValueType
	key       string
	count     int
	rules     models.ActionInputRules
}

func (a *ArrayTakeAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *ArrayTakeAction) GetKey() string {
	return a.key
}

func (a *ArrayTakeAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *ArrayTakeAction) GetOutputType() models.ActionValueType {
	return a.inputType
}

func (a *ArrayTakeAction) Execute(inputs ...any) (any, error) {
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

	if count > len(arr) {
		return arr, nil
	}

	return arr[:count], nil
}

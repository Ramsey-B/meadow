package array

import (
	"github.com/Gobusters/ectolinq"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var ArrayContainsRules = models.ActionInputRules{
	"array": {
		Type:              models.ValueTypeArray,
		Min:               1,
		Max:               1,
		IsItemTypeDynamic: true,
		Items: &models.ActionInputRule{
			Min: 0,
			Max: -1,
		},
	},
}

func NewArrayContainsAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(ArrayContainsRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ValidateArguments[ArrayContainsArgs](args)
	if err != nil {
		return nil, err
	}

	return &ArrayContainsAction{
		inputType: models.ActionValueType{
			Type:  models.ValueTypeArray,
			Items: models.ValueTypeAny,
		},
		args:  parsedArgs,
		key:   key,
		rules: rules,
	}, nil
}

type ArrayContainsArgs struct {
	SearchItems []any `json:"searchItems" validate:"omitempty"`
}

type ArrayContainsAction struct {
	key       string
	args      ArrayContainsArgs
	inputType models.ActionValueType
	rules     models.ActionInputRules
}

func (a *ArrayContainsAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *ArrayContainsAction) GetKey() string {
	return a.key
}

func (a *ArrayContainsAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *ArrayContainsAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{
		Type: models.ValueTypeBool,
	}
}

// find the array input
func (a *ArrayContainsAction) Execute(inputs ...any) (any, error) {
	if len(a.args.SearchItems) > 0 {
		inputs = append(inputs, a.args.SearchItems...)
	}

	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, err
	}

	arrArgs := ectolinq.First(actionInputs["array"].Value)
	arr, err := utils.AnyToType[[]any](arrArgs)
	if err != nil {
		return nil, err
	}

	searchItems := actionInputs["array"].Items
	for _, searchItem := range searchItems {
		if ectolinq.Contains(arr, searchItem) {
			return true, nil
		}
	}

	return false, nil
}

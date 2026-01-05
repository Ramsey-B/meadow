package array

import (
	"reflect"

	"github.com/Gobusters/ectolinq"
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var ArrayIndexOfRules = models.ActionInputRules{
	"array": {
		Type:              models.ValueTypeArray,
		Min:               1,
		Max:               1,
		IsItemTypeDynamic: true,
		Items: &models.ActionInputRule{
			Min: 0,
			Max: 1,
		},
	},
}

type ArrayIndexOfArguments struct {
	Item any `json:"item" validate:"omitempty"`
}

func NewArrayIndexOfAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(ArrayIndexOfRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ValidateArguments[ArrayIndexOfArguments](args)
	if err != nil {
		return nil, err
	}

	if parsedArgs.Item != nil {
		err := models.IsActionValueType(parsedArgs.Item, models.ActionValueType{
			Type: rules["array"].Items.Type,
		})
		if err != nil {
			return nil, errors.WrapMappingError(err).AddAction(key)
		}
	}

	return &ArrayIndexOfAction{
		inputType: models.ActionValueType{
			Type:  models.ValueTypeArray,
			Items: models.ValueTypeAny,
		},
		key:   key,
		args:  parsedArgs,
		rules: rules,
	}, nil
}

type ArrayIndexOfAction struct {
	inputType models.ActionValueType
	key       string
	args      ArrayIndexOfArguments
	rules     models.ActionInputRules
}

func (a *ArrayIndexOfAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *ArrayIndexOfAction) GetKey() string {
	return a.key
}

func (a *ArrayIndexOfAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *ArrayIndexOfAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{
		Type: models.ValueTypeNumber,
	}
}

func (a *ArrayIndexOfAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, err
	}

	arrArgs := ectolinq.First(actionInputs["array"].Value)
	arr, err := utils.AnyToType[[]any](arrArgs)
	if err != nil {
		return nil, err
	}

	items := actionInputs["array"].Items
	if a.args.Item != nil {
		items = append(items, a.args.Item)
	}

	searchItem := ectolinq.First(items)
	for i, item := range arr {
		if reflect.DeepEqual(item, searchItem) {
			return i, nil
		}
	}

	return -1, nil
}

package array

import (
	"github.com/Gobusters/ectolinq"
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var ArrayPushRules = models.ActionInputRules{
	"array": {
		Type:              models.ValueTypeArray,
		Min:               1,
		Max:               1,
		IsItemTypeDynamic: true,
		Items: &models.ActionInputRule{
			Min: 1,
			Max: -1,
		},
	},
}

type ArrayPushArguments struct {
	Items []any `json:"items" validate:"omitempty"`
}

func NewArrayPushAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(ArrayPushRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ValidateArguments[ArrayPushArguments](args)
	if err != nil {
		return nil, err
	}

	if len(parsedArgs.Items) > 0 {
		for _, item := range parsedArgs.Items {
			err := models.IsActionValueType(item, models.ActionValueType{
				Type: rules["array"].Items.Type,
			})
			if err != nil {
				return nil, errors.WrapMappingError(err).AddAction(key)
			}
		}
	}

	return &ArrayPushAction{
		inputType: models.ActionValueType{
			Type:  rules["array"].Type,
			Items: rules["array"].Items.Type,
		},
		key:   key,
		items: parsedArgs.Items,
		rules: rules,
	}, nil
}

type ArrayPushAction struct {
	inputType models.ActionValueType
	key       string
	items     []any
	rules     models.ActionInputRules
}

func (a *ArrayPushAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *ArrayPushAction) GetKey() string {
	return a.key
}

func (a *ArrayPushAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *ArrayPushAction) GetOutputType() models.ActionValueType {
	return a.inputType
}

func (a *ArrayPushAction) Execute(inputs ...any) (any, error) {
	if len(a.items) > 0 {
		inputs = append(inputs, a.items...)
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

	return append(arr, actionInputs["array"].Items...), nil
}

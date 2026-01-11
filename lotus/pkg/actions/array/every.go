package array

import (
	"github.com/Gobusters/ectolinq"
	"github.com/Ramsey-B/lotus/pkg/actions/registry"
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var ArrayEveryRules = models.ActionInputRules{
	"array": {
		Type: models.ValueTypeArray,
		Min:  1,
		Max:  1,
	},
}

func NewArrayEveryAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(ArrayEveryRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ValidateArguments[ArrayEveryArgs](args)
	if err != nil {
		return nil, err
	}

	// Get items type from the actual input type (not from rules)
	// inputTypes[0] is the array input with its items type
	itemsType := models.ValueTypeAny
	if len(inputTypes) > 0 && inputTypes[0].Items != "" {
		itemsType = inputTypes[0].Items
	}

	action, err := registry.GetAction(parsedArgs.ActionKey, parsedArgs.Args, models.ActionValueType{
		Type: itemsType,
	})
	if err != nil {
		return nil, err
	}

	actionOutputType := action.GetOutputType()
	if actionOutputType.Type != models.ValueTypeBool {
		return nil, errors.NewMappingErrorf("array every action requires a boolean output type, got %s", actionOutputType.Type)
	}

	return &ArrayEveryAction{
		inputType: models.ActionValueType{
			Type:  models.ValueTypeArray,
			Items: models.ValueTypeAny,
		},
		args:   parsedArgs,
		key:    key,
		action: action,
		rules:  rules,
	}, nil
}

type ArrayEveryArgs struct {
	ActionKey string `json:"actionKey" validate:"required"`
	Args      any    `json:"args" validate:"omitempty"`
}

type ArrayEveryAction struct {
	key       string
	args      ArrayEveryArgs
	inputType models.ActionValueType
	action    models.Action
	rules     models.ActionInputRules
}

func (a *ArrayEveryAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *ArrayEveryAction) GetKey() string {
	return a.key
}

func (a *ArrayEveryAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *ArrayEveryAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{
		Type: models.ValueTypeBool,
	}
}

// find the array input
func (a *ArrayEveryAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, err
	}

	arrArgs := ectolinq.First(actionInputs["array"].Value)
	arr, err := utils.AnyToType[[]any](arrArgs)
	if err != nil {
		return nil, err
	}

	for _, item := range arr {
		result, err := a.action.Execute(item)
		if err != nil {
			return nil, err
		}

		passed, err := utils.AnyToType[bool](result)
		if err != nil {
			return nil, errors.WrapMappingError(err).AddAction(a.key)
		}

		if !passed {
			return false, nil
		}
	}

	return true, nil
}

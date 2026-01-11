package array

import (
	"github.com/Gobusters/ectolinq"
	"github.com/Ramsey-B/lotus/pkg/actions/registry"
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var ArrayFilterRules = models.ActionInputRules{
	"array": {
		Type: models.ValueTypeArray,
		Min:  1,
		Max:  1,
	},
}

func NewArrayFilterAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(ArrayFilterRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ValidateArguments[ArrayFilterArgs](args)
	if err != nil {
		return nil, err
	}

	// Get items type from the actual input type
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
		return nil, errors.NewMappingErrorf("array filter action requires a boolean predicate, got %s", actionOutputType.Type)
	}

	return &ArrayFilterAction{
		inputType: inputTypes[0], // Preserve the input array type including items
		args:      parsedArgs,
		key:       key,
		action:    action,
		invert:    parsedArgs.Invert,
		rules:     rules,
	}, nil
}

type ArrayFilterArgs struct {
	ActionKey string `json:"actionKey" validate:"required"`
	Args      any    `json:"args" validate:"omitempty"`
	Invert    bool   `json:"invert" validate:"omitempty"` // If true, keep items that FAIL the predicate
}

type ArrayFilterAction struct {
	key       string
	args      ArrayFilterArgs
	inputType models.ActionValueType
	action    models.Action
	invert    bool
	rules     models.ActionInputRules
}

func (a *ArrayFilterAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *ArrayFilterAction) GetKey() string {
	return a.key
}

func (a *ArrayFilterAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *ArrayFilterAction) GetOutputType() models.ActionValueType {
	// Output is same type as input (filtered array)
	return a.inputType
}

func (a *ArrayFilterAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, err
	}

	arrArgs := ectolinq.First(actionInputs["array"].Value)
	arr, err := utils.AnyToType[[]any](arrArgs)
	if err != nil {
		return nil, err
	}

	filtered := make([]any, 0, len(arr))

	for _, item := range arr {
		result, err := a.action.Execute(item)
		if err != nil {
			return nil, err
		}

		passed, err := utils.AnyToType[bool](result)
		if err != nil {
			return nil, errors.WrapMappingError(err).AddAction(a.key)
		}

		// Apply invert if configured
		if a.invert {
			passed = !passed
		}

		if passed {
			filtered = append(filtered, item)
		}
	}

	return filtered, nil
}

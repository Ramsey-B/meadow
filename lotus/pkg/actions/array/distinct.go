package array

import (
	"github.com/Gobusters/ectolinq"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var ArrayDistinctRules = models.ActionInputRules{
	"array": {
		Type: models.ValueTypeArray,
		Min:  1,
		Max:  1,
	},
}

func NewArrayDistinctAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(ArrayDistinctRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	return &ArrayDistinctAction{
		key:       key,
		inputType: inputTypes[0],
		rules:     rules,
	}, nil
}

type ArrayDistinctAction struct {
	inputType models.ActionValueType
	key       string
	rules     models.ActionInputRules
}

func (a *ArrayDistinctAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *ArrayDistinctAction) GetKey() string {
	return a.key
}

func (a *ArrayDistinctAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *ArrayDistinctAction) GetOutputType() models.ActionValueType {
	return a.inputType
}

func (a *ArrayDistinctAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, err
	}

	arrArgs := ectolinq.First(actionInputs["array"].Value)
	arr, err := utils.AnyToType[[]any](arrArgs)
	if err != nil {
		return nil, err
	}

	existing := make(map[any]bool)
	distinct := make([]any, 0)
	for _, item := range arr {
		if _, ok := existing[item]; !ok {
			existing[item] = true
			distinct = append(distinct, item)
		}
	}
	return distinct, nil
}

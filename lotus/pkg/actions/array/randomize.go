package array

import (
	"math/rand/v2"

	"github.com/Gobusters/ectolinq"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var ArrayRandomizeRules = models.ActionInputRules{
	"array": {
		Type: models.ValueTypeArray,
		Min:  1,
		Max:  1,
	},
}

func NewArrayRandomizeAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(ArrayRandomizeRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	return &ArrayRandomizeAction{
		inputType: inputTypes[0],
		key:       key,
		rules:     rules,
	}, nil
}

type ArrayRandomizeAction struct {
	inputType models.ActionValueType
	key       string
	rules     models.ActionInputRules
}

func (a *ArrayRandomizeAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *ArrayRandomizeAction) GetKey() string {
	return a.key
}

func (a *ArrayRandomizeAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *ArrayRandomizeAction) GetOutputType() models.ActionValueType {
	return a.inputType
}

func (a *ArrayRandomizeAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, err
	}

	arrArgs := ectolinq.First(actionInputs["array"].Value)
	arr, err := utils.AnyToType[[]any](arrArgs)
	if err != nil {
		return nil, err
	}

	rand.Shuffle(len(arr), func(i, j int) {
		arr[i], arr[j] = arr[j], arr[i]
	})

	return arr, nil
}

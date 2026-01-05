package number

import (
	"math"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var NumberIsEvenRules = models.ActionInputRules{
	"number": {
		Type: models.ValueTypeNumber,
		Min:  1,
		Max:  1,
	},
}

func NewNumberIsEvenAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(NumberIsEvenRules, inputTypes...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &NumberIsEvenAction{
		key: key,
		inputType: models.ActionValueType{
			Type: models.ValueTypeNumber,
		},
		rules: rules,
	}, nil
}

type NumberIsEvenAction struct {
	inputType models.ActionValueType
	key       string
	rules     models.ActionInputRules
}

func (a *NumberIsEvenAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *NumberIsEvenAction) GetKey() string {
	return a.key
}

func (a *NumberIsEvenAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberIsEvenAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{
		Type: models.ValueTypeBool,
	}
}

func (a *NumberIsEvenAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	if len(actionInputs["number"].Value) == 0 {
		return false, nil
	}

	num, err := utils.AnyToType[float64](actionInputs["number"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	// A number is even if it's an integer and divisible by 2
	isInteger := num == math.Floor(num)
	if !isInteger {
		return false, nil
	}

	return int64(num)%2 == 0, nil
}


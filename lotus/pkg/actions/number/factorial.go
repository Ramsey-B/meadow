package number

import (
	"math"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var NumberFactorialRules = models.ActionInputRules{
	"number": {
		Type: models.ValueTypeNumber,
		Min:  1,
		Max:  1,
	},
}

func NewNumberFactorialAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(NumberFactorialRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	return &NumberFactorialAction{
		inputType: models.ActionValueType{
			Type: models.ValueTypeNumber,
		},
		key:   key,
		rules: rules,
	}, nil
}

type NumberFactorialAction struct {
	inputType models.ActionValueType
	key       string
	rules     models.ActionInputRules
}

func (a *NumberFactorialAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *NumberFactorialAction) GetKey() string {
	return a.key
}

func (a *NumberFactorialAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberFactorialAction) GetOutputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberFactorialAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	inputNum, err := utils.AnyToType[float64](actionInputs["number"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(0)
	}

	result := math.Gamma(inputNum + 1)

	if math.IsNaN(result) {
		return nil, errors.NewMappingErrorf("action '%s' factorial result is not a number, got %f", a.GetKey(), inputNum)
	}

	if math.IsInf(result, 0) {
		return nil, errors.NewMappingErrorf("action '%s' factorial result is infinite, got %f", a.GetKey(), inputNum)
	}

	return result, nil
}

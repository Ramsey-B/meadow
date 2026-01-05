package number

import (
	"math"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var NumberToPositiveRules = models.ActionInputRules{
	"number": {
		Type: models.ValueTypeNumber,
		Min:  1,
		Max:  1,
	},
}

func NewNumberToPositiveAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(NumberToPositiveRules, inputTypes...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &NumberToPositiveAction{
		key: key,
		inputType: models.ActionValueType{
			Type: models.ValueTypeNumber,
		},
		rules: rules,
	}, nil
}

type NumberToPositiveAction struct {
	key       string
	inputType models.ActionValueType
	rules     models.ActionInputRules
}

func (a *NumberToPositiveAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *NumberToPositiveAction) GetKey() string {
	return a.key
}

func (a *NumberToPositiveAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberToPositiveAction) GetOutputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberToPositiveAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	num, err := utils.AnyToType[float64](actionInputs["number"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(0)
	}

	return math.Abs(num), nil
}

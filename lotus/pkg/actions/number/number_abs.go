package number

import (
	"math"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var NumberAbsRules = models.ActionInputRules{
	"number": {
		Type: models.ValueTypeNumber,
		Min:  1,
		Max:  1,
	},
}

func NewNumberAbsAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(NumberAbsRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	return &NumberAbsAction{
		key:   key,
		rules: rules,
	}, nil
}

type NumberAbsAction struct {
	key   string
	rules models.ActionInputRules
}

func (a *NumberAbsAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *NumberAbsAction) GetKey() string {
	return a.key
}

func (a *NumberAbsAction) GetInputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeNumber}
}

func (a *NumberAbsAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeNumber}
}

func (a *NumberAbsAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	num, err := utils.AnyToType[float64](actionInputs["number"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	return math.Abs(num), nil
}


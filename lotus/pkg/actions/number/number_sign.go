package number

import (
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var NumberSignRules = models.ActionInputRules{
	"number": {
		Type: models.ValueTypeNumber,
		Min:  1,
		Max:  1,
	},
}

func NewNumberSignAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(NumberSignRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	return &NumberSignAction{
		key:   key,
		rules: rules,
	}, nil
}

type NumberSignAction struct {
	key   string
	rules models.ActionInputRules
}

func (a *NumberSignAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *NumberSignAction) GetKey() string {
	return a.key
}

func (a *NumberSignAction) GetInputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeNumber}
}

func (a *NumberSignAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeNumber}
}

func (a *NumberSignAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	num, err := utils.AnyToType[float64](actionInputs["number"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	if num > 0 {
		return 1.0, nil
	} else if num < 0 {
		return -1.0, nil
	}
	return 0.0, nil
}


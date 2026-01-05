package number

import (
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var NumberToNegativeRules = models.ActionInputRules{
	"number": {
		Type: models.ValueTypeNumber,
		Min:  1,
		Max:  1,
	},
}

func NewNumberToNegativeAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(NumberToNegativeRules, inputTypes...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &NumberToNegativeAction{
		key: key,
		inputType: models.ActionValueType{
			Type: models.ValueTypeNumber,
		},
		rules: rules,
	}, nil
}

type NumberToNegativeAction struct {
	key       string
	inputType models.ActionValueType
	rules     models.ActionInputRules
}

func (a *NumberToNegativeAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *NumberToNegativeAction) GetKey() string {
	return a.key
}

func (a *NumberToNegativeAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberToNegativeAction) GetOutputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberToNegativeAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	num, err := utils.AnyToType[float64](actionInputs["number"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(0)
	}

	return -num, nil
}

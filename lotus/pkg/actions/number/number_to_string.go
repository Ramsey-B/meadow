package number

import (
	"strconv"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var NumberToStringRules = models.ActionInputRules{
	"number": {
		Type: models.ValueTypeNumber,
		Min:  1,
		Max:  1,
	},
}

func NewNumberToStringAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(NumberToStringRules, inputTypes...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &NumberToStringAction{
		key: key,
		inputType: models.ActionValueType{
			Type: models.ValueTypeNumber,
		},
		rules: rules,
	}, nil
}

type NumberToStringAction struct {
	key       string
	inputType models.ActionValueType
	rules     models.ActionInputRules
}

func (a *NumberToStringAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *NumberToStringAction) GetKey() string {
	return a.key
}

func (a *NumberToStringAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberToStringAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{
		Type: models.ValueTypeString,
	}
}

func (a *NumberToStringAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	num, err := utils.AnyToType[float64](actionInputs["number"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(0)
	}

	return strconv.FormatFloat(num, 'f', -1, 64), nil
}

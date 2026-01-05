package number

import (
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var NumberCubeRules = models.ActionInputRules{
	"number": {
		Type: models.ValueTypeNumber,
		Min:  1,
		Max:  1,
	},
}

func NewNumberCubeAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(NumberCubeRules, inputTypes...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &NumberCubeAction{
		inputType: models.ActionValueType{
			Type: models.ValueTypeNumber,
		},
		key:   key,
		rules: rules,
	}, nil
}

type NumberCubeAction struct {
	inputType models.ActionValueType
	key       string
	rules     models.ActionInputRules
}

func (a *NumberCubeAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *NumberCubeAction) GetKey() string {
	return a.key
}

func (a *NumberCubeAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberCubeAction) GetOutputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberCubeAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	num, err := utils.AnyToType[float64](actionInputs["number"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(0)
	}

	return num * num * num, nil
}

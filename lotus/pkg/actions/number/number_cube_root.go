package number

import (
	"math"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var NumberCubeRootRules = models.ActionInputRules{
	"number": {
		Type: models.ValueTypeNumber,
		Min:  1,
		Max:  1,
	},
}

func NewNumberCubeRootAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(NumberCubeRootRules, inputTypes...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &NumberCubeRootAction{
		inputType: models.ActionValueType{
			Type: models.ValueTypeNumber,
		},
		key:   key,
		rules: rules,
	}, nil
}

type NumberCubeRootAction struct {
	inputType models.ActionValueType
	key       string
	rules     models.ActionInputRules
}

func (a *NumberCubeRootAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *NumberCubeRootAction) GetKey() string {
	return a.key
}

func (a *NumberCubeRootAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberCubeRootAction) GetInputBType() models.ActionValueType {
	return a.inputType
}

func (a *NumberCubeRootAction) GetOutputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberCubeRootAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	num, err := utils.AnyToType[float64](actionInputs["number"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(0)
	}

	return math.Cbrt(num), nil
}

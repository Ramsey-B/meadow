package number

import (
	"math"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var NumberFloorRules = models.ActionInputRules{
	"number": {
		Type: models.ValueTypeNumber,
		Min:  1,
		Max:  1,
	},
}

func NewNumberFloorAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(NumberFloorRules, inputTypes...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &NumberFloorAction{
		inputType: models.ActionValueType{
			Type: models.ValueTypeNumber,
		},
		key:   key,
		rules: rules,
	}, nil
}

type NumberFloorAction struct {
	inputType models.ActionValueType
	key       string
	rules     models.ActionInputRules
}

func (a *NumberFloorAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *NumberFloorAction) GetKey() string {
	return a.key
}

func (a *NumberFloorAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberFloorAction) GetOutputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberFloorAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	num, err := utils.AnyToType[float64](actionInputs["number"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(0)
	}

	return math.Floor(num), nil
}

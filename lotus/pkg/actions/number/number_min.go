package number

import (
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var NumberMinRules = models.ActionInputRules{
	"number": {
		Type: models.ValueTypeNumber,
		Min:  1,
		Max:  2,
	},
}

type NumberMinArguments struct {
	Min float64 `json:"min" validate:"required"`
}

func NewNumberMinAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(NumberMinRules, inputTypes...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	parsedArgs, err := utils.ParseArguments[NumberMinArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &NumberMinAction{
		inputType: models.ActionValueType{
			Type: models.ValueTypeNumber,
		},
		key:   key,
		min:   parsedArgs.Min,
		rules: rules,
	}, nil
}

type NumberMinAction struct {
	inputType models.ActionValueType
	key       string
	min       float64
	rules     models.ActionInputRules
}

func (a *NumberMinAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *NumberMinAction) GetKey() string {
	return a.key
}

func (a *NumberMinAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberMinAction) GetInputBType() models.ActionValueType {
	return a.inputType
}

func (a *NumberMinAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeBool}
}

func (a *NumberMinAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	num, err := utils.AnyToType[float64](actionInputs["number"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(0)
	}

	min := a.min
	if len(inputs) == 2 {
		min, err = utils.AnyToType[float64](inputs[1])
		if err != nil {
			return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(1)
		}
	}

	return num < min, nil
}

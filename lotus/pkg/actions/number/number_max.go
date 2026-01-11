package number

import (
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var NumberMaxRules = models.ActionInputRules{
	"number": {
		Type: models.ValueTypeNumber,
		Min:  1,
		Max:  2,
	},
}

func NewNumberMaxAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(NumberMaxRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ParseArguments[NumberMaxArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &NumberMaxAction{
		inputType: models.ActionValueType{
			Type: models.ValueTypeNumber,
		},
		key:   key,
		max:   parsedArgs.Max,
		rules: rules,
	}, nil
}

type NumberMaxArguments struct {
	Max float64 `json:"max" validate:"required"`
}

type NumberMaxAction struct {
	inputType models.ActionValueType
	key       string
	max       float64
	rules     models.ActionInputRules
}

func (a *NumberMaxAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *NumberMaxAction) GetKey() string {
	return a.key
}

func (a *NumberMaxAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberMaxAction) GetInputBType() models.ActionValueType {
	return a.inputType
}

func (a *NumberMaxAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{
		Type: models.ValueTypeBool,
	}
}

func (a *NumberMaxAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	num, err := utils.AnyToType[float64](actionInputs["number"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(0)
	}

	max := a.max
	if len(inputs) == 2 {
		max, err = utils.AnyToType[float64](inputs[1])
		if err != nil {
			return nil, errors.WrapMappingError(err).AddAction(a.key).AddItemIndex(1)
		}
	}

	return num > max, nil
}

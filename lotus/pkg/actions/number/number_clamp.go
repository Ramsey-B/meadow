package number

import (
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var NumberClampRules = models.ActionInputRules{
	"number": {
		Type: models.ValueTypeNumber,
		Min:  1,
		Max:  1,
	},
}

type NumberClampArguments struct {
	Min float64 `json:"min" validate:"required"` // Minimum value
	Max float64 `json:"max" validate:"required"` // Maximum value
}

func NewNumberClampAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(NumberClampRules, inputTypes...)
	if err != nil {
		return nil, err
	}

	parsedArgs, err := utils.ParseArguments[NumberClampArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &NumberClampAction{
		key:        key,
		parsedArgs: parsedArgs,
		rules:      rules,
	}, nil
}

type NumberClampAction struct {
	key        string
	parsedArgs NumberClampArguments
	rules      models.ActionInputRules
}

func (a *NumberClampAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *NumberClampAction) GetKey() string {
	return a.key
}

func (a *NumberClampAction) GetInputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeNumber}
}

func (a *NumberClampAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{Type: models.ValueTypeNumber}
}

func (a *NumberClampAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	num, err := utils.AnyToType[float64](actionInputs["number"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	if num < a.parsedArgs.Min {
		return a.parsedArgs.Min, nil
	}
	if num > a.parsedArgs.Max {
		return a.parsedArgs.Max, nil
	}
	return num, nil
}


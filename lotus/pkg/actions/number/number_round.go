package number

import (
	"math"

	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var NumberRoundRules = models.ActionInputRules{
	"number": {
		Type: models.ValueTypeNumber,
		Min:  1,
		Max:  1,
	},
}

type NumberRoundArguments struct {
	Precision int `json:"precision" validate:"required,min=0"`
}

func NewNumberRoundAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(NumberRoundRules, inputTypes...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	parsedArgs, err := utils.ParseArguments[NumberRoundArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &NumberRoundAction{
		inputType: models.ActionValueType{
			Type: models.ValueTypeNumber,
		},
		key:       key,
		precision: parsedArgs.Precision,
		rules:     rules,
	}, nil
}

type NumberRoundAction struct {
	inputType models.ActionValueType
	key       string
	precision int
	rules     models.ActionInputRules
}

func (a *NumberRoundAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *NumberRoundAction) GetKey() string {
	return a.key
}

func (a *NumberRoundAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberRoundAction) GetInputBType() models.ActionValueType {
	return a.inputType
}

func (a *NumberRoundAction) GetOutputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberRoundAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	num, err := utils.AnyToType[float64](actionInputs["number"].Value[0])
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	return math.Round(num*math.Pow(10, float64(a.precision))) / math.Pow(10, float64(a.precision)), nil
}

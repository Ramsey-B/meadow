package number

import (
	"github.com/Gobusters/ectolinq"
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var NumberMultiplyRules = models.ActionInputRules{
	"number": {
		Type: models.ValueTypeNumber,
		Min:  1,
		Max:  -1,
	},
}

type NumberMultiplyArguments struct {
	Multiplier *float64 `json:"multiplier" validate:"omitempty"`
}

func NewNumberMultiplyAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(NumberMultiplyRules, inputTypes...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	parsedArgs, err := utils.ParseArguments[NumberMultiplyArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &NumberMultiplyAction{
		inputType: models.ActionValueType{
			Type: models.ValueTypeNumber,
		},
		key:        key,
		multiplier: parsedArgs.Multiplier,
		rules:      rules,
	}, nil
}

type NumberMultiplyAction struct {
	inputType  models.ActionValueType
	key        string
	multiplier *float64
	rules      models.ActionInputRules
}

func (a *NumberMultiplyAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *NumberMultiplyAction) GetKey() string {
	return a.key
}

func (a *NumberMultiplyAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberMultiplyAction) GetOutputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberMultiplyAction) Execute(inputs ...any) (any, error) {
	actionInputs, err := a.GetInputRules().Validate(inputs...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(a.key)
	}

	numbers := ectolinq.Map(actionInputs["number"].Value, func(input any) float64 {
		num, err := utils.AnyToType[float64](input)
		if err != nil {
			return 0
		}
		return num
	})

	product := 1.0
	for _, num := range numbers {
		multiplier := ectolinq.Ternary(a.multiplier != nil, *a.multiplier, num)

		product *= multiplier
	}

	return product, nil
}

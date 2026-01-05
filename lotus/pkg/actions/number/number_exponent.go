package number

import (
	"math"

	"github.com/Gobusters/ectolinq"
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var NumberExponentRules = models.ActionInputRules{
	"number": {
		Type: models.ValueTypeNumber,
		Min:  1,
		Max:  2,
	},
}

type NumberExponentArguments struct {
	Exponent float64 `json:"exponent" validate:"omitempty,min=0"`
}

func NewNumberExponentAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(NumberExponentRules, inputTypes...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	parsedArgs, err := utils.ParseArguments[NumberExponentArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &NumberExponentAction{
		key: key,
		inputType: models.ActionValueType{
			Type: models.ValueTypeNumber,
		},
		exp:   parsedArgs.Exponent,
		rules: rules,
	}, nil
}

type NumberExponentAction struct {
	inputType models.ActionValueType
	key       string
	exp       float64
	rules     models.ActionInputRules
}

func (a *NumberExponentAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *NumberExponentAction) GetKey() string {
	return a.key
}

func (a *NumberExponentAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberExponentAction) GetOutputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberExponentAction) Execute(inputs ...any) (any, error) {
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

	exp := a.exp
	if len(numbers) == 2 {
		exp = numbers[1]
	}

	result := math.Pow(numbers[0], exp)

	return result, nil
}

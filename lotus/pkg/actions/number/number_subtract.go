package number

import (
	"github.com/Gobusters/ectolinq"
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var NumberSubtractRules = models.ActionInputRules{
	"number": {
		Type: models.ValueTypeNumber,
		Min:  1,
		Max:  -1,
	},
}

type NumberSubtractArguments struct {
	Value float64 `json:"value" validate:"omitempty"`
}

func NewNumberSubtractAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(NumberSubtractRules, inputTypes...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	parsedArgs, err := utils.ParseArguments[NumberSubtractArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &NumberSubtractAction{
		key: key,
		inputType: models.ActionValueType{
			Type: models.ValueTypeNumber,
		},
		value: parsedArgs.Value,
		rules: rules,
	}, nil
}

type NumberSubtractAction struct {
	key       string
	inputType models.ActionValueType
	value     float64
	rules     models.ActionInputRules
}

func (a *NumberSubtractAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *NumberSubtractAction) GetKey() string {
	return a.key
}

func (a *NumberSubtractAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberSubtractAction) GetOutputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberSubtractAction) Execute(inputs ...any) (any, error) {
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

	numbers = append(numbers, a.value)

	result := ectolinq.First(numbers)
	for _, num := range numbers[1:] {
		result -= num
	}

	return result, nil
}

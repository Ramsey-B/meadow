package number

import (
	"github.com/Gobusters/ectolinq"
	"github.com/Ramsey-B/lotus/pkg/errors"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/lotus/pkg/utils"
)

var NumberEqualsRules = models.ActionInputRules{
	"number": {
		Type: models.ValueTypeNumber,
		Min:  1,
		Max:  -1,
	},
}

type NumberEqualsArguments struct {
	Value *float64 `json:"value" validate:"omitempty"`
}

func NewNumberEqualsAction(key string, args any, inputTypes ...models.ActionValueType) (models.Action, error) {
	rules, err := models.ValidateInputTypes(NumberEqualsRules, inputTypes...)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	parsedArgs, err := utils.ParseArguments[NumberEqualsArguments](args)
	if err != nil {
		return nil, errors.WrapMappingError(err).AddAction(key)
	}

	return &NumberEqualsAction{
		key: key,
		inputType: models.ActionValueType{
			Type: models.ValueTypeNumber,
		},
		value: parsedArgs.Value,
		rules: rules,
	}, nil
}

type NumberEqualsAction struct {
	inputType models.ActionValueType
	key       string
	value     *float64
	rules     models.ActionInputRules
}

func (a *NumberEqualsAction) GetInputRules() models.ActionInputRules {
	return a.rules
}

func (a *NumberEqualsAction) GetKey() string {
	return a.key
}

func (a *NumberEqualsAction) GetInputType() models.ActionValueType {
	return a.inputType
}

func (a *NumberEqualsAction) GetInputBType() models.ActionValueType {
	return a.inputType
}

func (a *NumberEqualsAction) GetOutputType() models.ActionValueType {
	return models.ActionValueType{
		Type: models.ValueTypeBool,
	}
}

func (a *NumberEqualsAction) Execute(inputs ...any) (any, error) {
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

	if a.value != nil {
		numbers = append([]float64{*a.value}, numbers...)
	}

	expected := ectolinq.First(numbers)
	for _, num := range numbers[1:] {
		if num != expected {
			return false, nil
		}
	}

	return true, nil
}
